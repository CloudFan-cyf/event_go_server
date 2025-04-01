本仓库为go编写的用于机器人紧急事件上报的http+websocket服务器。
- 对机器人侧，机器人使用http POST上报json序列化的事件/状态数据
- 对用户侧，用户使用websocket连接，在初次连接时获取服务器缓存的数据，并在之后持续接收最新的数据
# Getting Started
创建并使用docker-compose来启动容器：
```docker-compose.yaml
version: '3.8'

services:
  robot-server:
    build: .
    ports:
      - "80:80"
    restart: always
    environment:
      - TZ=Asia/Shanghai
      - PORT=80 # 使用PORT环境变量来指定服务器程序监听的端口
    volumes:
      - ./config:/app/config # 如果需要配置文件可以挂载
```
在项目根目录下，运行：
```
docker-compose build
```
来构建镜像，随后运行以下命令启动容器
```
docker-compose up
```
此时容器应该已经创建并启动，监听环境变量指定的端口

# 机器人侧数据接口
该 HTTP 服务器提供了两个主要的接口供机器人客户端上报数据和事件。以下是详细的接口说明：
## 1. 上报紧急事件接口

### URL
```
POST /robot/event
```

### 描述
机器人客户端可以通过此接口上报紧急事件，例如故障或异常信息。
### 请求头
- `Content-Type: application/json`
### 请求体
请求体为 JSON 格式，包含以下字段：

| 字段名       | 类型     | 必填 | 描述                     |
|--------------|----------|------|--------------------------|
| `robot_id`   | `uint32` | 是   | 机器人唯一标识符         |
| `robot_name` | `string` | 是   | 机器人名称               |
| `message`    | `object` | 是   | 事件的详细信息（JSON 格式） |

#### 示例请求
```json
POST /robot/event HTTP/1.1
Host: localhost:8081
Content-Type: application/json

{
    "robot_id": 1,
    "robot_name": "Robot-A",
    "message": {
        "location": "机械臂",
        "error": "关节卡顿",
        "severity": "critical"
    }
}
```

### 响应
- **成功**：返回 HTTP 状态码 `201 Created`。
- **失败**：返回相应的错误状态码和错误信息。
## 2. 上报状态数据接口

### URL
```
POST /robot/data
```

### 描述
机器人客户端可以通过此接口上报实时状态数据，例如速度、位置、电量等。

### 请求头
- `Content-Type: application/json`

### 请求体
请求体为 JSON 格式，包含以下字段：

| 字段名        | 类型     | 必填 | 描述                     |
|---------------|----------|------|--------------------------|
| `robot_id`    | `uint32` | 是   | 机器人唯一标识符         |
| `robot_name`  | `string` | 是   | 机器人名称               |
| `status_type` | `string` | 是   | 状态类型（如 `speed`、`position` 等） |
| `data`        | `string` | 是   | 状态数据的具体值         |

#### 示例请求
```json
POST /robot/data HTTP/1.1
Host: localhost:8081
Content-Type: application/json

{
    "robot_id": 1,
    "robot_name": "Robot-A",
    "status_type": "speed",
    "data": "5.2"
}
```

### 响应
- **成功**：返回 HTTP 状态码 `201 Created`。
- **失败**：返回相应的错误状态码和错误信息。
## 注意事项

1. **服务器端口**：默认监听地址为 `http://localhost:80`。
2. **数据格式**：确保请求体为合法的 JSON 格式。
3. **错误处理**：如果请求格式不正确或字段缺失，服务器会返回 `400 Bad Request`。
4. **并发支持**：服务器支持多客户端并发上报数据。
