package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// 配置文件格式
type Config struct {
	Port      int `json:"port"`
	ZoneID    int `json:"zone_id"`
	DcID      int `json:"dc_id"`
	MachineID int `json:"machine_id"`
}

// 请求格式
type Request struct {
	CMD   uint8
	Count uint8
}

const MAX_COUNTER = 1 << 16

// Bucket 结构
type Bucket struct {
	timestamp int64
	zoneID    int
	dcID      int
	machineID int
	counter   uint16
	raw       uint64
	lock      sync.Mutex
}

// 创建一个新的 Bucket
func NewBucket(zoneID, dcID, machineID int) *Bucket {
	b := &Bucket{
		timestamp: time.Now().Unix(),
		zoneID:    zoneID,
		dcID:      dcID,
		machineID: machineID,
		counter:   0,
	}
	b.raw = uint64(b.timestamp)<<32 | uint64(b.zoneID)<<27 | uint64(b.dcID)<<24 | uint64(b.machineID)<<21
	return b
}

// 从 Bucket 获取一个 ID
func (b *Bucket) GetID() uint64 {
	b.lock.Lock()
	defer b.lock.Unlock()

	// 生成 ID
	id := b.raw | uint64(b.counter)

	// 更新计数器
	b.counter++

	return id
}

// 从 Bucket 获取多个 ID
func (b *Bucket) GetIDs(count uint8) []uint64 {
	b.lock.Lock()
	defer b.lock.Unlock()

	ids := make([]uint64, count)

	for i := range ids {
		ids[i] = b.raw | uint64(b.counter)
		b.counter++
		if b.counter >= MAX_COUNTER {
			break
		}
	}

	return ids
}

func (b *Bucket) Refresh(t int64) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.timestamp = t
	b.raw = uint64(b.timestamp)<<32 | uint64(b.zoneID)<<27 | uint64(b.dcID)<<24 | uint64(b.machineID)<<21
}

// 创建 TCP 服务
func createServer(port int, buckets []*Bucket) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("Failed to accept connection: %s", err)
			continue
		}

		// 为每个新的连接分配一个随机的 Bucket
		bucket := buckets[rand.Intn(len(buckets))]

		go handleConnection(conn, bucket)
	}
}

// 处理连接
func handleConnection(conn net.Conn, bucket *Bucket) {

	defer func() {
		if e := recover(); e != nil {
			log.Error("recover_panic")
		}
	}()

	defer conn.Close()

	for {
		req := &Request{}
		err := binary.Read(conn, binary.BigEndian, req)
		if err != nil {
			if err != io.EOF {
				log.Error("Failed to read request:", err)
			}
			return
		}

		switch req.CMD {
		case 0:
			id := bucket.GetID()
			binary.Write(conn, binary.BigEndian, &id)
		case 1:
			ids := bucket.GetIDs(req.Count)
			binary.Write(conn, binary.BigEndian, &req.Count)
			for _, id := range ids {
				binary.Write(conn, binary.BigEndian, &id)
			}
		default:
			log.Infof("Unknown CMD:%x", req.CMD)
			return
		}
	}
}

func main() {
	configFile := flag.String("config", "config.json", "Path to config file")
	flag.Parse()

	// 读取配置文件
	config := &Config{}
	file, err := os.Open(*configFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(config)
	if err != nil {
		panic(err)
	}

	// 校验参数
	if config.ZoneID > 31 || config.DcID > 7 || config.MachineID > 7 {
		log.Error("Invalid config values.")
		return
	}

	// 创建 Bucket
	buckets := make([]*Bucket, 32)
	for i := range buckets {
		buckets[i] = NewBucket(config.ZoneID, config.DcID, config.MachineID)
	}

	// 创建 TCP 服务
	go createServer(config.Port, buckets)

	log.Infof("idGen server start at %d", config.Port)

	// 更新 Bucket 时间戳
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		for _, bucket := range buckets {
			bucket.Refresh(time.Now().Unix())
		}
	}
}
