package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func insertUsersForPaginationTest(t *testing.T, total int) {
	t.Helper()
	for id := 1; id <= total; id++ {
		user := &User{
			Id:          id,
			Username:    fmt.Sprintf("user%02d", id),
			Password:    "password123",
			DisplayName: fmt.Sprintf("User %02d", id),
			Email:       fmt.Sprintf("user%02d@example.com", id),
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Group:       "default",
			AffCode:     fmt.Sprintf("aff%02d", id),
		}
		require.NoError(t, DB.Create(user).Error)
	}
}

func collectUserIDs(users []*User) []int {
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	return ids
}

func TestGetAllUsersSortsBeforePagination(t *testing.T) {
	truncateTables(t)
	insertUsersForPaginationTest(t, 42)

	pageOne, total, err := GetAllUsers(&common.PageInfo{Page: 1, PageSize: 20}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), total)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, collectUserIDs(pageOne))

	pageTwo, total, err := GetAllUsers(&common.PageInfo{Page: 2, PageSize: 20}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), total)
	assert.Equal(t, []int{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}, collectUserIDs(pageTwo))

	pageThree, total, err := GetAllUsers(&common.PageInfo{Page: 3, PageSize: 20}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), total)
	assert.Equal(t, []int{41, 42}, collectUserIDs(pageThree))
}

func TestSearchUsersSortsBeforePagination(t *testing.T) {
	truncateTables(t)
	insertUsersForPaginationTest(t, 42)

	users, total, err := SearchUsers("user", "", nil, nil, 20, 20, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), total)
	assert.Equal(t, []int{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}, collectUserIDs(users))
}

func TestSearchUsersMatchesDepartmentKeywordDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}

			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)

			previousDB, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				assert.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&User{}))

			versionQuery := "SELECT version()"
			if dialect == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database version: %s", version)

			// Isolate the fixture from other data in the shared test databases.
			const group = "p45-dept-search"
			require.NoError(t, db.Unscoped().Where(commonGroupCol+" = ?", group).Delete(&User{}).Error)

			// The shared PostgreSQL fixture keeps a legacy manual id column, so assign ids explicitly.
			var maxID int64
			require.NoError(t, db.Raw("SELECT COALESCE(MAX(id), 0) FROM users").Scan(&maxID).Error)

			fixtures := []struct {
				username   string
				department string
			}{
				{"p45-video-a", "视频部"},
				{"p45-video-b", "视频部"},
				{"p45-video-c", "视频部"},
				{"p45-video-d", "视频部"},
				{"p45-other-a", "测试组"},
				{"p45-other-b", "研发部"},
			}
			created := make([]*User, 0, len(fixtures))
			for i, fixture := range fixtures {
				user := &User{
					Id:          int(maxID) + i + 1,
					Username:    fixture.username,
					DisplayName: "P45 " + fixture.username,
					Email:       fixture.username + "@example.com",
					Department:  fixture.department,
					Group:       group,
					AffCode:     fixture.username,
					Role:        common.RoleCommonUser,
					Status:      common.UserStatusEnabled,
				}
				require.NoError(t, db.Create(user).Error)
				created = append(created, user)
			}
			t.Cleanup(func() {
				for _, user := range created {
					assert.NoError(t, db.Unscoped().Delete(user).Error)
				}
			})

			matchingIDs := []int{created[0].Id, created[1].Id, created[2].Id, created[3].Id}
			users, total, err := SearchUsers("视频部", group, nil, nil, 0, 2, NewUserSortOptions("id", "asc"))
			require.NoError(t, err)
			require.Equal(t, int64(4), total)
			assert.Equal(t, []int{matchingIDs[0], matchingIDs[1]}, collectUserIDs(users))
			for _, user := range users {
				assert.Equal(t, "视频部", user.Department)
			}

			users, total, err = SearchUsers("视频部", group, nil, nil, 2, 2, NewUserSortOptions("id", "asc"))
			require.NoError(t, err)
			require.Equal(t, int64(4), total)
			assert.Equal(t, []int{matchingIDs[2], matchingIDs[3]}, collectUserIDs(users))

			users, total, err = SearchUsers("视频部", group, nil, nil, 0, 2, NewUserSortOptions("id", "desc"))
			require.NoError(t, err)
			require.Equal(t, int64(4), total)
			assert.Equal(t, []int{matchingIDs[3], matchingIDs[2]}, collectUserIDs(users))

			users, total, err = SearchUsers("p45-other-a", group, nil, nil, 0, 10, NewUserSortOptions("id", "asc"))
			require.NoError(t, err)
			require.Equal(t, int64(1), total)
			assert.Equal(t, []int{created[4].Id}, collectUserIDs(users))
		})
	}
}
