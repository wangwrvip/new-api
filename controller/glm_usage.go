package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const glmUsagePath = "/api/monitor/usage/quota/limit"

const (
	glmUsageDefaultBase = "https://open.bigmodel.cn"
	// Same limits as the reference glm-usage.py script: batches stay small
	// and upstream fetches run with a bounded worker pool.
	maxGlmUsageBatchIds = 50
	glmUsageConcurrency = 8
	glmUsageTimeout     = 15 * time.Second
)

// glmUsageChannelResult is the per-channel entry of the batch response:
// glmUsageData on success, Message alone on failure.
type glmUsageChannelResult struct {
	glmUsageData
	Message string `json:"message,omitempty"`
}

// glmUsageSlot is one token-limit window (5-hour rolling or weekly).
type glmUsageSlot struct {
	UsedPercent      int   `json:"used_pct"`
	RemainingPercent int   `json:"remaining_pct"`
	NextResetMs      int64 `json:"next_reset_ms"`
}

// glmUsageData is the parsed GLM Coding Plan usage returned to the frontend.
type glmUsageData struct {
	Success bool          `json:"success"`
	Level   string        `json:"level,omitempty"`
	FiveH   *glmUsageSlot `json:"five_h,omitempty"`
	Weekly  *glmUsageSlot `json:"weekly,omitempty"`
}

type glmUsageLimitEntry struct {
	Type          string  `json:"type"`
	Unit          int     `json:"unit"`
	Number        int     `json:"number"`
	Percentage    float64 `json:"percentage"`
	NextResetTime int64   `json:"nextResetTime"`
}

type glmUsageUpstreamResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Data    struct {
		Level  string               `json:"level"`
		Limits []glmUsageLimitEntry `json:"limits"`
	} `json:"data"`
}

// The two failure classes of parseGlmUsageResponse. They are matched with
// errors.Is so the handler can return a stable, differentiated message: an
// upstream business rejection versus a malformed response body.
var (
	glmUsageRejected        = errors.New("upstream rejected the request")
	glmUsageInvalidResponse = errors.New("upstream response invalid")
)

// parseGlmUsageResponse classifies TOKENS_LIMIT windows: unit 6 is the
// weekly quota, unit 3 with number 5 is the 5-hour rolling window; with an
// unknown unit, a window resetting more than 24h away is treated as weekly
// (the same heuristic as the reference glm-usage.py script). TIME_LIMIT
// entries track per-call quotas and are ignored.
func parseGlmUsageResponse(body []byte, nowMs int64) (glmUsageData, error) {
	var upstream glmUsageUpstreamResponse
	if err := common.Unmarshal(body, &upstream); err != nil {
		return glmUsageData{}, fmt.Errorf("%w: %v", glmUsageInvalidResponse, err)
	}
	if !upstream.Success {
		msg := strings.TrimSpace(upstream.Msg)
		if msg == "" {
			msg = fmt.Sprintf("code %d", upstream.Code)
		}
		return glmUsageData{}, fmt.Errorf("%w: %s", glmUsageRejected, msg)
	}

	data := glmUsageData{
		Success: true,
		Level:   strings.ToUpper(strings.TrimSpace(upstream.Data.Level)),
	}
	dayMs := 24 * int64(time.Hour/time.Millisecond)
	for _, lim := range upstream.Data.Limits {
		if lim.Type != "TOKENS_LIMIT" || lim.NextResetTime <= 0 {
			continue
		}
		weekly := false
		switch {
		case lim.Unit == 6:
			weekly = true
		case lim.Unit == 3 && lim.Number == 5:
		case lim.NextResetTime-nowMs > dayMs:
			weekly = true
		}
		used := int(math.Round(lim.Percentage))
		used = min(max(used, 0), 100)
		entry := &glmUsageSlot{
			UsedPercent:      used,
			RemainingPercent: 100 - used,
			NextResetMs:      lim.NextResetTime,
		}
		slot := &data.FiveH
		if weekly {
			slot = &data.Weekly
		}
		if *slot == nil {
			*slot = entry
		}
	}
	return data, nil
}

// fetchGlmChannelUsage queries the GLM Coding Plan usage endpoint with the
// channel key. The usage host is derived from the channel base URL origin
// (open.bigmodel.cn / api.z.ai), falling back to the mainland default.
func fetchGlmChannelUsage(ctx context.Context, channel *model.Channel) glmUsageChannelResult {
	usageBase := glmUsageDefaultBase
	if baseURL := strings.TrimSpace(channel.GetBaseURL()); baseURL != "" {
		if parsed, err := url.Parse(baseURL); err == nil && parsed.Host != "" {
			usageBase = parsed.Scheme + "://" + parsed.Host
		}
	}

	ctx, cancel := context.WithTimeout(ctx, glmUsageTimeout)
	defer cancel()

	// Use the channel transport settings so proxy, TLS, and HTTP policy match
	// what the relay path uses for this channel.
	setting := channel.GetSetting()
	client, err := service.GetHttpClientWithProxySettings(setting.Proxy, setting)
	if err != nil {
		common.SysError(common.LogText("GLM usage: channel %d: failed to build http client: %s", channel.Id, err.Error()))
		return glmUsageChannelResult{Message: "http client setup failed"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageBase+glmUsagePath, nil)
	if err != nil {
		common.SysError(common.LogText("GLM usage: channel %d: failed to build request: %s", channel.Id, err.Error()))
		return glmUsageChannelResult{Message: "request build failed"}
	}
	req.Header.Set("Authorization", strings.TrimSpace(channel.Key))
	req.Header.Set("Accept-Language", "en-US,en")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		common.SysError(common.LogText("GLM usage: channel %d: upstream request failed: %s", channel.Id, err.Error()))
		return glmUsageChannelResult{Message: "Upstream request failed"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		common.SysError(common.LogText("GLM usage: channel %d: unexpected upstream status %d", channel.Id, resp.StatusCode))
		return glmUsageChannelResult{Message: fmt.Sprintf("upstream status: %d", resp.StatusCode)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		common.SysError(common.LogText("GLM usage: channel %d: failed to read upstream response: %s", channel.Id, err.Error()))
		return glmUsageChannelResult{Message: "upstream response read failed"}
	}
	data, err := parseGlmUsageResponse(body, time.Now().UnixMilli())
	if err != nil {
		common.SysError(common.LogText("GLM usage: channel %d: invalid upstream response: %s", channel.Id, err.Error()))
		gatewayMessage := "upstream response invalid"
		if errors.Is(err, glmUsageRejected) {
			gatewayMessage = "upstream rejected the request"
		}
		return glmUsageChannelResult{Message: gatewayMessage}
	}
	return glmUsageChannelResult{glmUsageData: data}
}

// GetGlmChannelsUsage returns the GLM Coding Plan usage (5h / weekly token
// windows) for the requested channel ids: GET /api/channel/glm/usage?ids=1,2,3.
func GetGlmChannelsUsage(c *gin.Context) {
	seen := make(map[int]bool)
	var ids []int
	for _, part := range strings.Split(c.Query("ids"), ",") {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		if len(ids) >= maxGlmUsageBatchIds {
			break
		}
	}
	if len(ids) == 0 {
		common.ApiError(c, errors.New("no valid channel ids in query parameter"))
		return
	}

	results := make(map[string]glmUsageChannelResult, len(ids))
	sem := make(chan struct{}, glmUsageConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := func() glmUsageChannelResult {
				channel, err := model.GetChannelById(id, true)
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return glmUsageChannelResult{Message: "channel not found"}
					}
					common.SysError(common.LogText("GLM usage: failed to load channel %d: %s", id, err.Error()))
					return glmUsageChannelResult{Message: "channel lookup failed"}
				}
				if channel == nil {
					return glmUsageChannelResult{Message: "channel not found"}
				}
				if channel.Type != constant.ChannelTypeZhipu && channel.Type != constant.ChannelTypeZhipu_v4 {
					return glmUsageChannelResult{Message: "channel type is not Zhipu"}
				}
				if channel.ChannelInfo.IsMultiKey {
					return glmUsageChannelResult{Message: "multi-key channel is not supported"}
				}
				return fetchGlmChannelUsage(c.Request.Context(), channel)
			}()

			mu.Lock()
			results[strconv.Itoa(id)] = result
			mu.Unlock()
		}(id)
	}
	wg.Wait()

	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}
