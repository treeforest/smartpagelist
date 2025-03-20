package smartpagelist

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockStateStore 模拟区块链状态存储
type mockStateStore struct {
	mu    sync.Mutex
	store map[string][]byte
}

func NewMockStateStore() StateStore {
	return &mockStateStore{
		store: make(map[string][]byte),
	}
}

func (m *mockStateStore) PutState(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
	return nil
}

func (m *mockStateStore) GetState(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, exists := m.store[key]
	if !exists {
		return nil, nil
	}
	return value, nil
}

// ------------------------------ 测试用例 ------------------------------

func TestNewList(t *testing.T) {
	store := NewMockStateStore()
	list := NewList("test_list", 10, store)

	// 验证初始元数据
	meta, err := list.getMeta()
	if err != nil {
		t.Fatalf("getMeta failed: %v", err)
	}

	if meta.TotalCount != 0 || meta.LastPageNumber != 0 {
		t.Errorf("expected empty meta, got %+v", meta)
	}
}

func TestPushBackSingleElement(t *testing.T) {
	store := NewMockStateStore()
	list := NewList("test_list", 10, store)

	// 添加元素
	if err := list.PushBack("item1"); err != nil {
		t.Fatalf("PushBack failed: %v", err)
	}

	// 验证元数据
	meta, _ := list.getMeta()
	if meta.TotalCount != 1 || meta.LastPageNumber != 1 {
		t.Errorf("meta mismatch: expected TotalCount=1, LastPageNumber=1, got %+v", meta)
	}

	// 验证分页数据
	page1, err := list.GetPage(1)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}
	if len(page1) != 1 || page1[0] != "item1" {
		t.Errorf("page data mismatch: %v", page1)
	}
}

func TestPushBackFullPage(t *testing.T) {
	store := NewMockStateStore()
	list := NewList("test_list", 3, store) // 每页3元素

	// 填满第一页
	for i := 0; i < 3; i++ {
		if err := list.PushBack(fmt.Sprintf("item%d", i+1)); err != nil {
			t.Fatal(err)
		}
	}

	// 添加第四元素（应创建第二页）
	if err := list.PushBack("item4"); err != nil {
		t.Fatal(err)
	}

	// 验证元数据
	meta, _ := list.getMeta()
	if meta.TotalCount != 4 || meta.LastPageNumber != 2 {
		t.Errorf("meta mismatch: expected TotalCount=4, LastPageNumber=2, got %+v", meta)
	}

	// 验证分页数据
	t.Run("Page1", func(t *testing.T) {
		page, _ := list.GetPage(1)
		if len(page) != 3 {
			t.Errorf("expected 3 items in page1, got %v", page)
		}
	})

	t.Run("Page2", func(t *testing.T) {
		page, _ := list.GetPage(2)
		if len(page) != 1 || page[0] != "item4" {
			t.Errorf("page2 data mismatch: %v", page)
		}
	})
}

func TestGetPageInvalidNumber(t *testing.T) {
	store := NewMockStateStore()
	list := NewList("test_list", 10, store)

	// 空列表查询
	_, err := list.GetPage(1)
	if !errors.Is(err, ErrPageNotFound) {
		t.Errorf("expected ErrPageNotFound, got %v", err)
	}

	// 添加元素后查询越界页
	_ = list.PushBack("item1")
	_, err = list.GetPage(2)
	if !errors.Is(err, ErrPageNotFound) {
		t.Errorf("expected ErrPageNotFound, got %v", err)
	}
}

func TestLength(t *testing.T) {
	store := NewMockStateStore()
	list := NewList("test_list", 3, store)

	// 空列表
	if length, _ := list.Length(); length != 0 {
		t.Errorf("expected length 0, got %d", length)
	}

	// 添加元素
	itemsToAdd := 5
	for i := 0; i < itemsToAdd; i++ {
		_ = list.PushBack(fmt.Sprintf("item%d", i+1))
	}

	// 验证长度
	if length, _ := list.Length(); length != itemsToAdd {
		t.Errorf("expected length %d, got %d", itemsToAdd, length)
	}
}

func TestRange(t *testing.T) {
	store := NewMockStateStore()
	list := NewList("test_list", 3, store)

	// 添加测试数据
	for i := 0; i < 5; i++ {
		_ = list.PushBack(fmt.Sprintf("item%d", i+1))
	}

	t.Run("FullRange", func(t *testing.T) {
		var count int
		err := list.Range(0, -1, func(index int, value string) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if count != 5 {
			t.Errorf("expected 5 items, got %d", count)
		}
	})

	t.Run("PartialRange", func(t *testing.T) {
		expected := []string{"item3", "item4"}
		var results []string
		err := list.Range(2, 4, func(index int, value string) error {
			results = append(results, value)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("expected %v, got %v", expected, results)
		}
	})
}

func TestGetOperations(t *testing.T) {
	// 初始化测试环境
	store := NewMockStateStore()
	list := NewList("test_list", 3, store) // 分页大小3

	// 准备测试数据：4个元素，分2页（第1页3元素，第2页1元素）
	testData := []string{"item1", "item2", "item3", "item4"}
	for _, item := range testData {
		if err := list.PushBack(item); err != nil {
			t.Fatalf("初始化数据失败: %v", err)
		}
	}

	t.Run("Get正常用例", func(t *testing.T) {
		testCases := []struct {
			name     string
			index    int
			expected string
		}{
			{"首元素", 0, "item1"},
			{"第一页末尾", 2, "item3"},
			{"跨页元素", 3, "item4"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := list.Get(tc.index)
				if err != nil || result != tc.expected {
					t.Errorf("Get(%d) => (%q, %v), 预期 (%q, nil)",
						tc.index, result, err, tc.expected)
				}
			})
		}
	})

	t.Run("Get异常用例", func(t *testing.T) {
		testCases := []struct {
			name        string
			index       int
			expectedErr error
		}{
			{"负数索引", -1, ErrIndexOutOfRange},
			{"越界索引", 4, ErrIndexOutOfRange},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := list.Get(tc.index)
				if !errors.Is(err, tc.expectedErr) {
					t.Errorf("Get(%d) 预期错误 %v, 实际得到 %v",
						tc.index, tc.expectedErr, err)
				}
			})
		}
	})

	t.Run("Get数据不一致用例", func(t *testing.T) {
		// 手动构造损坏数据：第二页标记存在但实际为空
		corruptStore := NewMockStateStore()
		corruptList := NewList("corrupt_list", 3, corruptStore)

		// 设置元数据表示有1页3元素
		corruptMeta := &listMeta{
			LastPageNumber: 1,
			TotalCount:     3,
		}
		metaData, _ := json.Marshal(corruptMeta)
		_ = corruptStore.PutState(corruptList.metaKey(), metaData)

		// 实际存储空页数据
		_ = corruptStore.PutState(corruptList.buildPageKey(1), []byte("[]"))

		// 验证获取索引2时报错
		_, err := corruptList.Get(2)
		if err == nil || !strings.Contains(err.Error(), "data inconsistency") {
			t.Errorf("预期数据不一致错误，实际得到: %v", err)
		}
	})

	t.Run("GetLast正常用例", func(t *testing.T) {
		// 获取最后一个元素
		last, err := list.GetLast()
		if err != nil || last != "item4" {
			t.Errorf("GetLast() => (%q, %v), 预期 ('item4', nil)", last, err)
		}

		// 添加新元素后验证
		_ = list.PushBack("item5")
		last, _ = list.GetLast()
		if last != "item5" {
			t.Errorf("GetLast() 更新后预期 'item5', 得到 %q", last)
		}
	})

	t.Run("GetLast异常用例", func(t *testing.T) {
		// 空列表用例
		emptyList := NewList("empty_list", 3, store)
		_, err := emptyList.GetLast()
		if !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("空列表预期 ErrIndexOutOfRange, 实际得到 %v", err)
		}

		// 构造最后一页为空的情况
		badMetaStore := NewMockStateStore()
		badList := NewList("bad_list", 3, badMetaStore)

		// 元数据标记有1页但实际无数据
		badMeta := &listMeta{
			LastPageNumber: 1,
			TotalCount:     1,
		}
		metaData, _ := json.Marshal(badMeta)
		_ = badMetaStore.PutState(badList.metaKey(), metaData)

		// 验证数据损坏检测
		_, err = badList.GetLast()
		if err == nil {
			t.Errorf("预期数据损坏错误，实际得到: %v", err)
		}
	})
}

// 压测参数配置
const (
	TotalItems    = 100000 // 总测试数据量
	SmallPageSize = 10     // 小分页配置
	LargePageSize = 1000   // 大分页配置
	SamplePoints  = 100    // 采样点数量
)

// 初始化测试列表
func initList(pageSize int) (*List, StateStore) {
	store := NewMockStateStore()
	return NewList("perf_test", pageSize, store), store
}

// ------------------------------ 插入性能测试 ------------------------------

func BenchmarkInsert_SmallPage(b *testing.B) {
	benchmarkInsert(b, SmallPageSize)
}

func BenchmarkInsert_LargePage(b *testing.B) {
	benchmarkInsert(b, LargePageSize)
}

func benchmarkInsert(b *testing.B, pageSize int) {
	list, _ := initList(pageSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 批量插入测试(每次测试迭代插入TotalItems个元素)
		start := time.Now()
		for n := 0; n < TotalItems; n++ {
			_ = list.PushBack(strconv.Itoa(n))
		}
		b.ReportMetric(float64(time.Since(start).Milliseconds())/float64(TotalItems), "ms/op")
	}
}

// ------------------------------ 查询性能测试 ------------------------------

func BenchmarkQuery_SmallPage(b *testing.B) {
	benchmarkQuery(b, SmallPageSize)
}

func BenchmarkQuery_LargePage(b *testing.B) {
	benchmarkQuery(b, LargePageSize)
}

func benchmarkQuery(b *testing.B, pageSize int) {
	list, _ := initList(pageSize)
	prepareTestData(list, TotalItems)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 随机查询不同位置的元素
		start := time.Now()
		for n := 0; n < SamplePoints; n++ {
			index := n * (TotalItems / SamplePoints)
			_, _ = list.Get(index)
		}
		b.ReportMetric(float64(time.Since(start).Milliseconds())/float64(SamplePoints), "ms/op")
	}
}

// ------------------------------ 遍历性能测试 ------------------------------

func BenchmarkIterate_SmallPage(b *testing.B) {
	benchmarkIterate(b, SmallPageSize)
}

func BenchmarkIterate_LargePage(b *testing.B) {
	benchmarkIterate(b, LargePageSize)
}

func benchmarkIterate(b *testing.B, pageSize int) {
	list, _ := initList(pageSize)
	prepareTestData(list, TotalItems)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		_ = list.Range(0, -1, func(_ int, _ string) error {
			return nil
		})
		b.ReportMetric(float64(time.Since(start).Milliseconds()), "ms/op")
	}
}

// ------------------------------ 工具函数 ------------------------------

// 准备测试数据
func prepareTestData(list *List, count int) {
	for i := 0; i < count; i++ {
		_ = list.PushBack("data-" + strconv.Itoa(i))
	}
}
