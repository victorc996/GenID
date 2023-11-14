# GenID
A 64-bit distributed globally unique ID generation service, using a snowflake-like algorithm.

[中文文档](./README.zh.md)

## Features
1. Independent TCP service, simple protocol, and language agnostic.
2. Simple operation and maintenance, low maintenance cost.
3. Supports obtaining a single ID or multiple IDs at once.
4. High performance, 32 token buckets, supports millions of connections, and can generate up to 4 billion IDs per second.
5. Uses long connections, allocates token buckets fixedly, greatly ensuring that the client obtains IDs in a monotonically increasing manner, optimizing database storage performance.

## 64-bit ID Description

 0                      32 37     40 43     48       64
+------------------------+--------+----+-----+--------+
|       Timestamp        | ZoneID |DcID|MachineID|BucketID+Counter|
+------------------------+--------+----+-----+--------+
|<--- 32 bits --->|<-- 5 bits -->|<-- 3 bits -->|<-- 3 bits -->|<-- 5 bits -->|<-- 16 bits -->|

- Timestamp: High 32 bits, represents the current timestamp accurate to seconds.
- ZoneID: 5 bits, represents the zone ID, globally divided into 32 regions.
- DcID: 3 bits, represents the data center ID, with support for 8 data centers per region.
- MachineID: 3 bits, represents the machine ID, with a maximum of 8 GenID nodes deployable per data center.
- BucketID+Counter: 5 bits for bucket ID and 16 bits for the counter. The token bucket is used to increase concurrency, and the counter is used to generate unique IDs on a single bucket.

The high 32 bits of the ID use the timestamp, which is beneficial for troubleshooting. `id<<32` can obtain the time of ID generation.

## Deployment
1. Compilation

```bash
go build -o genid main.go
```

2. Execution

```
nohup genid -config config.json  >>run.log 2>&1  &
```

## Client Usage

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
        # CMD=0 indicates to get a single ID
        self.sock.sendall(struct.pack('BB', 0, 0))
        data = self.sock.recv(8)
        return struct.unpack('>Q', data)[0]

    def get_ids(self, count):
        # CMD=1 indicates to get multiple IDs, Count indicates the number of IDs to obtain
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
``````

## High Availability
Supports two methods

1. The client program passes a set of GenID service addresses, and the client performs failover and automatic reconnection.
2. Server-side load balancing, deploying GenID behind 4th layer proxies such as nginx/haproxy.

## Expansion

To improve system reliability, the following features need to be implemented:

1. When the program starts, mutually check the configuration accuracy of each GenID node in the same data center to prevent data contamination.

2. Periodically write the timestamp to disk. When a node recovers from hardware failure, it can compare the server time with the stored timestamp to avoid causing rollback.