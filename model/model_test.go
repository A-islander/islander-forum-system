package model

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
)

// func TestGetIndex(t *testing.T) {
// 	fmt.Println(GetForumPostIndex(1, 0, 10))
// }

// func TestGetList(t *testing.T) {
// 	fmt.Println(GetForumPostList(1, 0, 2))
// }

// func TestUpdate(t *testing.T) {
// 	// UpdateForumPostCount(1, int(time.Now().Unix()))
// }

// func TestUser(t *testing.T) {
// 	// fmt.Println(GetUserById(2), GetUserByToken("233"), GetUserArr([]int{1, 2, 3}))
// 	fmt.Println(GetUserArr([]int{1, 2, 3}))
// }

// func TestGetLastList(t *testing.T) {
// 	fmt.Println(GetLastPostList([]int{0, 1}, 2))
// }

func TestGetBuff(t *testing.T) {
	fmt.Println(GetForumPostIndexBuff(1, 0, 10))
}

func TestGetReplyBuff(t *testing.T) {
	fmt.Println(GetLastPostListBuff([]int{1, 18, 24, 28, 40, 43}, 5))
	rdb := newRdb()
	res, _ := rdb.Keys(ctx, "*").Result()
	fmt.Println(res)
}

func TestLastTime(t *testing.T) {
	fmt.Println(GetForumIndexLastTime(0, 10, []int{}))
}

func TestChangePost(t *testing.T) {
	ChangePostPlate(57, 1)
	ChangeFollowPostPlate(57, 1)
}

func TestRdbAddCount(t *testing.T) {
	val := AddCount("123", 1, 10)
	fmt.Println(val)
}

// TestSageAddConcurrent 验证 S3 修复：N 个不同 userId 并发投票，
// 最终 addArr 必须包含全部 N 个 userId（无丢失），且无重复。
// 需真实 DB + 一个已存在帖子，通过环境变量 ISLANDER_SAGE_TEST_POST 指定帖子 id；未设置则跳过。
func TestSageAddConcurrent(t *testing.T) {
	postIdStr := os.Getenv("ISLANDER_SAGE_TEST_POST")
	if postIdStr == "" {
		t.Skip("set ISLANDER_SAGE_TEST_POST=<postId> to run sage concurrency test")
	}
	postId, err := strconv.Atoi(postIdStr)
	if err != nil {
		t.Fatalf("invalid ISLANDER_SAGE_TEST_POST: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		userId := 900000 + i // 测试用 userId 段，避免与真实用户冲突
		go func() {
			defer wg.Done()
			SageAddPostUser(postId, userId)
		}()
	}
	wg.Wait()

	addArr, _ := readSageArr(postId)
	// 去重校验
	seen := make(map[int]bool, len(addArr))
	for _, uid := range addArr {
		if uid < 900000 || uid >= 900000+n {
			continue // 跳过历史数据
		}
		if seen[uid] {
			t.Fatalf("duplicate userId in sageAddId: %d", uid)
		}
		seen[uid] = true
	}
	if len(seen) != n {
		t.Fatalf("sage votes lost: expected %d, got %d (addArr=%v)", n, len(seen), addArr)
	}
	t.Logf("OK: all %d concurrent votes persisted, no loss, no dup", n)

	// 清理：把这些测试 userId 全部移除
	for uid := range seen {
		SageRemovePostUser(postId, uid)
	}
}
