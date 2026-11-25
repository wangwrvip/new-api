package common

import (
	"strings"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetResponsePreviewStoresTextUnderLimitVerbatim(t *testing.T) {
	info := &RelayInfo{}

	info.SetResponsePreview("short answer")

	require.Equal(t, "short answer", info.ResponsePreview)
}

func TestSetResponsePreviewTruncatesAsciiToMaxRunes(t *testing.T) {
	info := &RelayInfo{}

	info.SetResponsePreview(strings.Repeat("a", 3000))

	require.Equal(t, 1024, len(info.ResponsePreview))
	assert.Equal(t, strings.Repeat("a", 1024), info.ResponsePreview)
}

func TestSetResponsePreviewTruncatesMultiByteTextOnRuneBoundary(t *testing.T) {
	info := &RelayInfo{}

	// 每个汉字占 3 个字节；按字符截断应保留 1024 个汉字，且结果仍是合法 UTF-8。
	info.SetResponsePreview(strings.Repeat("汉", 3000))

	require.Equal(t, 1024, len([]rune(info.ResponsePreview)))
	assert.Equal(t, strings.Repeat("汉", 1024), info.ResponsePreview)
}

func TestSetResponsePreviewKeepsBoundaryLengthText(t *testing.T) {
	info := &RelayInfo{}

	info.SetResponsePreview(strings.Repeat("中", 1024))

	require.Equal(t, strings.Repeat("中", 1024), info.ResponsePreview)
}

func TestSetResponsePreviewSkipsWhenConsumeLogDisabled(t *testing.T) {
	enabled := rootcommon.LogConsumeEnabled
	t.Cleanup(func() { rootcommon.LogConsumeEnabled = enabled })
	rootcommon.LogConsumeEnabled = false

	info := &RelayInfo{}
	info.SetResponsePreview("some output")

	assert.Empty(t, info.ResponsePreview)
}
