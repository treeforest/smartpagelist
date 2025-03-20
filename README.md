# 区块链智能合约分页列表库

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

专为区块链智能合约设计的分页列表实现，支持高效的大规模数据管理，兼容主流区块链状态数据库。

## 功能特性

- 📃 ​**自动分页存储**：数据按固定页大小自动拆分存储
- 📊 ​**元数据管理**：实时跟踪总数据量和最新页码
- ⚡ ​**区块链优化**：针对链码状态数据库操作特别设计
- 🔒 ​**原子性保证**：基于区块链状态数据库的事务特性
- 🔍 ​**灵活查询**：支持范围遍历和直接页码访问

## 安装方式

```bash
go get github.com/treeforest/smartpagelist
```

## 快速入门

### 基础操作示例

```go
import (
    "fmt"
    "github.com/treeforest/smart-page-list"
)

// 初始化区块链状态存储
store := NewBlockchainStateStore() // 需实现StateStore接口
list := smartPageList.NewList("用户记录", 10, store)

// 添加元素
err := list.PushBack("用户A")
if err != nil {
    panic("区块链状态更新失败: " + err.Error())
}

// 分页查询
page1, err := list.GetPage(1)
if err != nil {
    panic("分页查询失败: " + err.Error())
}
fmt.Println("第一页数据:", page1) // ["用户A"]
```

### 数据遍历示例

```go
// 遍历第5-14号元素（索引从0开始）
err := list.Range(5, 15, func(索引 int, 值 string) error {
    fmt.Printf("索引 %d: %s\n", 索引, 值)
    return nil
})
if err != nil {
    panic("数据遍历失败: " + err.Error())
}
```

## 性能建议

### 分页大小选择参考

| 分页大小 | 适用场景         | 存储影响 |
|----------|------------------|----------|
| 10-50    | 高频写入系统     | 中等     |
| 50-100   | 读取密集型应用   | 较高     |
| 100+     | 批量数据处理     | 显著     |

## 授权许可

Apache 许可证 2.0 版本 - 详见 [LICENSE](https://www.apache.org/licenses/LICENSE-2.0.txt)