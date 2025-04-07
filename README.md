# SimpCache 项目文档

## 1. 项目概述

SimpCache 是一个分布式缓存系统，支持本地缓存和远程缓存的协作。它通过一致性哈希算法实现负载均衡，并使用 LRU 算法管理本地缓存，同时利用 `singleflight` 模块避免重复加载相同的数据。

### 核心功能
- **本地缓存**：基于 LRU（Least Recently Used）算法管理缓存数据。
- **远程缓存**：通过 HTTP 协议与远程节点交互，支持分布式缓存。
- **一致性哈希**：用于在多个节点之间分配缓存键。
- **并发控制**：通过 `sync.Mutex` 和 `singleflight` 模块确保线程安全。
- **接口设计**：提供 RESTful API 接口供外部访问缓存数据。

---

## 2. 核心模块分析

### 2.1 SingleFlight 模块

#### 功能
`singleflight` 模块用于避免重复执行相同的任务。当多个 goroutine 同时请求同一个键时，`singleflight` 只会执行一次加载逻辑，其他请求会等待结果返回。

#### 实现细节
- **核心结构**：
  - `Group`：维护一个 `map[string]*call`，用于存储正在执行的任务。
  - `call`：表示一个正在执行或已完成的任务，包含结果值和错误信息。
- **关键方法**：
  - `Do(key string, fn func() (interface{}, error))`：根据键执行任务，确保同一键的任务只执行一次。
- **优势**：
  - 减少重复加载的开销。
  - 提高系统的性能和稳定性。

#### 文件位置
- `simpchache/singleflight/singleflight.go`

---

### 2.2 一致性哈希模块

#### 功能
一致性哈希模块用于将缓存键分配到不同的节点上，确保负载均衡并减少节点增减时的数据迁移。

#### 实现细节
- **核心结构**：
  - `Map`：包含虚拟节点的数量（`replicas`）、哈希函数（`hash`）、有序的哈希值数组（`keys`）以及哈希值与实际键的映射（`hashMap`）。
- **关键方法**：
  - `New(replicas int, fn Hash)`：创建一个新的一致性哈希实例。
  - `Add(keys ...string)`：添加一组键到一致性哈希中。
  - `Get(key string)`：根据输入键找到最接近的节点。
- **虚拟节点**：
  - 每个实际节点会被复制多次（由 `replicas` 决定），以提高负载均衡效果。
- **二分查找**：
  - 使用 `sort.Search` 快速定位最近的节点。

#### 文件位置
- `simpchache/consistenthash/consistenthash.go`

---

### 2.3 LRU 缓存模块

#### 功能
LRU 模块实现了本地缓存的内存管理，确保缓存大小不超过设定的限制。

#### 实现细节
- **核心结构**：
  - `Cache`：包含最大缓存大小（`maxBytes`）、当前占用大小（`nbytes`）、双向链表（`ll`）以及键值对的映射（`cache`）。
  - `entry`：表示缓存中的一个条目，包含键和值。
- **关键方法**：
  - `New(maxBytes int64, onEvicted func(string, Value))`：创建一个新的 LRU 缓存实例。
  - `Add(key string, value Value)`：添加一个键值对到缓存中，超出限制时移除最旧的条目。
  - `Get(key string)`：获取指定键的值，若存在则将其移动到链表头部。
  - `RemoveOldest()`：移除最旧的缓存条目。
- **回调机制**：
  - 当缓存条目被移除时，可以触发 `OnEvicted` 回调函数。

#### 文件位置
- `simpchache/lru/lru.go`

---

## 3. 系统架构

### 3.1 组件关系

| 组件         | 描述                                                         |
|--------------|--------------------------------------------------------------|
| `Group`      | 缓存的核心组件，负责管理本地缓存和远程缓存的协作。           |
| `HTTPPool`   | 实现了一致性哈希的 PeerPicker 接口，用于选择远程节点。       |
| `ByteView`   | 提供一种安全的方式来操作和访问字节数据，防止外部修改。        |
| `LRU Cache`  | 管理本地缓存的内存使用，确保高效的数据访问和淘汰策略。       |
| `SingleFlight` | 避免重复加载相同的数据，提高系统的性能和稳定性。            |

### 3.2 数据流

1. **客户端请求**：
   - 客户端通过 API 接口（`/api`）请求缓存数据。
2. **本地缓存检查**：
   - `Group` 检查本地缓存是否命中。如果命中，直接返回数据。
3. **远程缓存加载**：
   - 如果本地缓存未命中，`Group` 调用 `PeerPicker` 的 `PickPeer` 方法选择远程节点。
   - 远程节点通过 HTTP 协议返回数据。
4. **数据加载与缓存**：
   - 如果远程缓存也未命中，`Group` 调用 `Getter` 加载数据，并更新本地缓存。

---

## 4. 示例代码

### 4.1 启动缓存服务器

```go
func startCacheServer(addr string, addrs []string, gee *simpchache.Group) {
	peers := simpchache.NewHTTPPool(addr)
	peers.Set(addrs...)
	gee.RegisterPeers(peers)
	log.Println("simpchache is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}
```

### 4.2 启动 API 服务器

```go
func startAPIServer(apiAddr string, gee *simpchache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := gee.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())
		}))
	log.Println("fontend server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}
```

---

## 5. 总结

SimpCache 是一个功能完善的分布式缓存系统，结合了 LRU、一致性哈希和 `singleflight` 等技术，提供了高效的缓存管理和负载均衡能力。通过 RESTful API 接口，外部应用可以方便地访问缓存数据。
