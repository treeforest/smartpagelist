package smartpagelist

import (
	"encoding/json"
	"errors"
	"fmt"
)

// StateStore 定义状态存储接口，用于与区块链状态数据库交互
type StateStore interface {
	PutState(key string, value []byte) error
	GetState(key string) ([]byte, error)
}

// List 实现基于状态数据库的分页列表，适用于智能合约场景
type List struct {
	key      string     // 列表唯一标识
	pageSize int        // 每页最大元素数量
	store    StateStore // 状态存储接口
}

// 错误类型定义
var (
	ErrPageNotFound    = errors.New("page not found")
	ErrIndexOutOfRange = errors.New("index out of range")
)

const (
	DefaultPageSize = 10
)

// ------------------------------ 初始化方法 ------------------------------

// NewList 创建分页列表实例
//   - listKey: 列表唯一标识，用于状态数据库中的键名前缀
//   - pageSize: 每页元素数量（建议值：10~100，根据业务场景调整）
//   - store: 状态存储实现（需由合约上下文注入）
func NewList(listKey string, pageSize int, store StateStore) *List {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	return &List{
		key:      listKey,
		pageSize: pageSize,
		store:    store,
	}
}

// ------------------------------ 元数据管理 ------------------------------

// listMeta 存储列表的元数据
type listMeta struct {
	LastPageNumber int `json:"lastPageNumber"` // 最新页码（从1开始）
	TotalCount     int `json:"totalCount"`     // 列表总元素数量
}

// metaKey 生成元数据存储键（格式: "listKey_meta"）
func (l *List) metaKey() string {
	return fmt.Sprintf("%s_meta", l.key)
}

// getMeta 从状态数据库读取元数据
func (l *List) getMeta() (*listMeta, error) {
	metaBytes, err := l.store.GetState(l.metaKey())
	if err != nil {
		return nil, fmt.Errorf("get meta state failed [key:%s]: %w", l.metaKey(), err)
	}

	// 初始化空元数据
	if len(metaBytes) == 0 {
		return &listMeta{}, nil
	}

	var meta listMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal meta failed [key:%s]: %w", l.metaKey(), err)
	}
	return &meta, nil
}

// saveMeta 将元数据写入状态数据库
func (l *List) saveMeta(meta *listMeta) error {
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta failed: %w", err)
	}

	if err := l.store.PutState(l.metaKey(), metaBytes); err != nil {
		return fmt.Errorf("save meta state failed [key:%s]: %w", l.metaKey(), err)
	}
	return nil
}

// ------------------------------ 分页操作 ------------------------------

// PushBack 追加元素到列表末尾
// 注意：此操作会修改元数据和分页数据，确保原子性由底层状态存储保证
func (l *List) PushBack(value string) error {
	meta, err := l.getMeta()
	if err != nil {
		return err
	}

	// 计算目标页码（新增页或当前页）
	targetPage := meta.LastPageNumber
	if meta.TotalCount%l.pageSize == 0 {
		targetPage++
	}

	// 读取或初始化当前页数据
	pageKey := l.buildPageKey(targetPage)
	pageData, err := l.store.GetState(pageKey)
	if err != nil {
		return fmt.Errorf("get page state failed [key:%s]: %w", pageKey, err)
	}

	var values []string
	if len(pageData) > 0 {
		if err := json.Unmarshal(pageData, &values); err != nil {
			return fmt.Errorf("unmarshal page data failed [key:%s]: %w", pageKey, err)
		}
	}

	// 追加元素并保存
	values = append(values, value)
	newPageData, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("marshal new page data failed: %w", err)
	}

	if err := l.store.PutState(pageKey, newPageData); err != nil {
		return fmt.Errorf("save page state failed [key:%s]: %w", pageKey, err)
	}

	// 更新元数据
	meta.LastPageNumber = targetPage
	meta.TotalCount++
	return l.saveMeta(meta)
}

// GetPage 获取指定页码的元素列表
//   - pageNumber: 页码（从1开始）
//   - 返回: 当前页元素列表，或 ErrPageNotFound
func (l *List) GetPage(pageNumber int) ([]string, error) {
	if pageNumber < 1 {
		return nil, errors.New("page number must be >= 1")
	}

	meta, err := l.getMeta()
	if err != nil {
		return nil, err
	}
	if pageNumber > meta.LastPageNumber {
		return nil, ErrPageNotFound
	}

	pageKey := l.buildPageKey(pageNumber)
	pageData, err := l.store.GetState(pageKey)
	if err != nil {
		return nil, fmt.Errorf("get page state failed [key:%s]: %w", pageKey, err)
	}

	var values []string
	if err := json.Unmarshal(pageData, &values); err != nil {
		return nil, fmt.Errorf("unmarshal page data failed [key:%s]: %w", pageKey, err)
	}
	return values, nil
}

// ------------------------------ 查询方法 ------------------------------

// Length 获取列表总元素数量
func (l *List) Length() (int, error) {
	meta, err := l.getMeta()
	if err != nil {
		return 0, err
	}
	return meta.TotalCount, nil
}

// Range 遍历指定索引范围内的元素
//   - start: 起始索引（包含，从0开始）
//   - end: 结束索引（不包含，-1表示列表末尾）
//   - fn: 遍历回调函数（返回 error 可提前终止遍历）
func (l *List) Range(start, end int, fn func(index int, value string) error) error {
	meta, err := l.getMeta()
	if err != nil {
		return err
	}

	// 参数校验
	if end == -1 {
		end = meta.TotalCount
	}
	if start < 0 || end > meta.TotalCount || start >= end {
		return ErrIndexOutOfRange
	}

	currentPage := (start / l.pageSize) + 1
	startPos := start % l.pageSize

	for index := start; index < end; {
		values, err := l.GetPage(currentPage)
		if err != nil {
			return err
		}

		for i := startPos; i < len(values) && index < end; i++ {
			if err := fn(index, values[i]); err != nil {
				return err
			}
			index++
		}

		currentPage++
		startPos = 0 // 后续页从索引0开始
	}

	return nil
}

// ------------------------------ 工具方法 ------------------------------

// buildPageKey 生成分页数据存储键（格式: "listKey_page_页码"）
func (l *List) buildPageKey(pageNumber int) string {
	return fmt.Sprintf("%s_page_%d", l.key, pageNumber)
}
