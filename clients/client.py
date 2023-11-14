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