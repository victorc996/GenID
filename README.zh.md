# GenID
64位分布式全局唯一ID生成服务，采用类似雪花算法(snowflake)

## 特性
2. 独立的TCP服务，协议简单，和编程语言无关
2. 运维部署简单，维护成本低
2. 支持一次获取单个ID 或者多个ID
2. 高性能，32路令牌桶，支持百万连接，每秒最多生成 40亿个ID
2. 使用长连接，固定分配令牌桶，极大概率保证了客户端获取id单调递增，优化数据库存储性能

## 64位ID说明

 0                      32 37     40 43     48       64
+------------------------+--------+----+-----+--------+
|       Timestamp        | ZoneID |DcID|MachineID|BucketID+Counter|
+------------------------+--------+----+-----+--------+
|<--- 32 bits --->|<-- 5 bits -->|<-- 3 bits -->|<-- 3 bits -->|<-- 5 bits -->|<-- 16 bits -->|

- Timestamp（时间戳）: 高32位，表示当前时间戳，精确到秒
- ZoneID（区域ID）: 5位，表示区域ID，全球可以划分32个区域
- DcID（数据中心ID）: 3位，表示数据中心ID，每个区域支持8个数据中心
- MachineID（机器ID）: 3位，表示机器ID，每个数据中心最多可以部署8个GenID节点
- BucketID+Counter（桶ID+计数器）: 5位桶ID+16位计数器，令牌桶用于提高并发度，计时器用于在单个桶上生成唯一ID

ID高32位采用时间戳，有利于排查问题，`id<<32` 即可得到ID生成的时间

## 部署
1. 编译

```
go build -o genid main.go
```

2. 运行

```
nohup genid -config config.json  >>run.log 2>&1  &
```

## 客户端使用
下面是一个python client

```python

import socket
import struct

class IDGeneratorClient:
    def __init__(self, host, port):
        self.host = host
        self.port = port
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.sock.connect((self.host, self.port))

    def get_id(self):
        # CMD=0表示获取一个ID
        self.sock.sendall(struct.pack('BB', 0, 0))
        data = self.sock.recv(8)
        return struct.unpack('>Q', data)[0]

    def get_ids(self, count):
        # CMD=1表示获取多个ID，Count表示需要获取的ID数量
        self.sock.sendall(struct.pack('BB', 1, count))
        head = self.sock.recv(1)
        num = struct.unpack(">B",head)[0]
        
        data=b''
        target_len = 8 * num
        while True:
            data += self.sock.recv(target_len)
            if len(data) >= target_len:
                ids = struct.unpack('>'+'Q' * num, data)
                return ids

    def close(self):
        self.sock.close()
        
if __name__ == "__main__":
    client = IDGeneratorClient('localhost', 3778)

    for i in range(100):
        ids = client.get_ids(20)
        print(ids)

    client.close()

```


## 高可用
支持两种方式
1. 客户端程序传入一组GenID服务地址，由客户端做故障转移,失败重连
2. 服务端做负载均衡，把GenID部署在nginx/haproxy 等4层代理之后


## 扩展
为了提高系统可靠性，下面的特性还需要实现

1. 程序启动的时候，互相检查同一个DC中各个GenID节点配置准确性，防止数据污染
2. 定时把时间戳写盘，当节点从硬件故障中恢复的时候，可以对比服务器时间和存储的时间戳，避免产生回拨