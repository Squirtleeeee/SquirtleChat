# WebSocket 协议

URL: `ws://localhost:8081/ws?token=<access_token>&device_id=<device_id>`

## 帧格式 (JSON)
```json
{"type":"ping|pong|ack|message|error","payload":{}}
```

## 上行 message
```json
{
  "type": "message",
  "payload": {
    "client_msg_id": "uuid",
    "conversation_id": "string",
    "conversation_type": 1,
    "msg_type": 1,
    "content": "text or file_ref json"
  }
}
```

## 下行 message
```json
{
  "type": "message",
  "payload": {
    "msg_id": 123,
    "client_msg_id": "uuid",
    "conversation_id": "string",
    "from_user_id": 1,
    "seq": 100,
    "msg_type": 1,
    "content": "...",
    "created_at": 1710000000
  }
}
```

## 枚举
- conversation_type: 1=单聊 2=群聊
- msg_type: 1=文本 2=图片 3=文件 4=系统

## 心跳
客户端每 30s 发 ping，服务端 pong。90s 无 ping 断连。

## ACK
服务端处理成功后回 ack: `{client_msg_id, msg_id, seq, status}`
