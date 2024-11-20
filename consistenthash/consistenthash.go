package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

var (
	defaultReplicas = 50
	defaultHash     = crc32.ChecksumIEEE
)

type Hash func(data []byte) uint32

type Map struct {
	hash     Hash
	replicas int   //虚拟节点倍数
	keys     []int //sorted
	hashMap  map[int]string
}

type ConsOptions func(*Map)

func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil { // 默认散列函数为crc32
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

func (m *Map) Register(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

// 选择节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	return m.hashMap[m.keys[idx%len(m.keys)]]
}

func (m *Map) Remove(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			delete(m.hashMap, hash)
		}
	}
	newKeys := make([]int, 0, len(m.hashMap)) // 重建哈希环
	for key := range m.hashMap {
		newKeys = append(newKeys, key)
	}
	m.keys = newKeys
	sort.Ints(m.keys)

}
