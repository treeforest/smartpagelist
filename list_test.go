package smartpagelist

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
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
