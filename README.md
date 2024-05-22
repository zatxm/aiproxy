AI Chat集成转发、代理
==========

golang版ai chat工具集成，目前支持openai、bing、claude、gemini和coze，后续陆续增加

# 使用

**1. 源码构建**

```
git clone https://github.com/zatxm/aiproxy
cd aiproxy
go build -ldflags "-s -w" -o aiproxy cmd/main.go
./aiproxy -c /whereis/c.yaml
```

**2. docker**

* 本地构建

```
git clone https://github.com/zatxm/aiproxy
cd aiproxy
docker build -t zatxm120/aiproxy . # 镜像名称可以自行定义
```

* docker库

```
docker pull zatxm120/aiproxy
```

* 启用

```
docker run -d --name aiproxy --restart --net=host always zatxm120/aiproxy #自身网络，默认配置端口8999
docker run -d --name aiproxy --restart always -p 8084:8999 zatxm120/aiproxy #映射端口，默认8999,实际还是要看配置文件的端口号
docker run -d --name aiproxy --restart always -p 8084:8999 -v /your-app-data:/anp/data zatxm120/aiproxy #映射文件夹，包含配置文件等
```

## 配置说明
**1. 创建数据目录**

假设放在/opt/aiproxy目录上，目录根据自己需求创建

```
mkdir -p /opt/aiproxy/ssl #放置https证书
mkdir -p /opt/aiproxy/cookies #放置openai chat登录cookie文件
mkdir -p /opt/aiproxy/hars #放置openai登录har
mkdir -p /opt/aiproxy/pics #放置验证码，一般没用到
mkdir -p /opt/aiproxy/etc #配置文件目录，配置文件复制到该目录
```
**2. 配置文件c.yaml**

源码中的etc/c.yaml为配置实例，复制到/opt/aiproxy/etc目录下修改

**3. docker映射目录**

```
docker run -d --name aiproxy --restart always -p 8084:8999 -v /opt/aiproxy:/anp/data zatxm120/aiproxy
```

## 接口应用说明

**1. 通用接口/c/v1/chat/completions**

支持通信头部header加入密钥(不加随机获取配置文件的密钥)，bing暂时不需要，一般以下两种：

* **Authorization**：通用
* **x-auth-id**：对应配置密钥ID，根据此值获取配置文件中设置的密钥

```
curl -X POST http://192.168.0.1:8999/c/v1/chat/completions -d '{
    "messages": [
        {
            "role": "user",
            "content": "你是谁？"
        }
    ],
    "model": "text-davinci-002-render-sha",
    "provider": "openai-chat-web"
}'
```

provider参数说明如下：

* **openai-chat-web**：openai web chat,支持免登录(有IP要求，一般美国IP就行)

  额外参数：

  ```
  {
      ...
      "openai": {
          "conversation": {
              "conversation_id": "697b28e8-e228-4abb-b356-c8ccdccf82f3",
              "index": "1000001",
              "parent_message_id": "dd6d9561-bebe-42da-91c6-f8fde6b105d9"
          },
          "message_id": "697b28e8-e228-4abb-b356-c8ccdccf82f4",
          "arkose_token": "arkose token"
      }
  }
  ```
  * index为密钥ID,头部x-auth-id优先级高于index
  * 表示在同一会话基础上进行对话，此值在任一对话通信后会返回

  额外返回：

  ```
  {
      ...
      "openai": {
          "conversation_id": "697b28e8-e228-4abb-b356-c8ccdccf82f3",
          "index": "1000001",
          "parent_message_id": "dd6d9561-bebe-42da-91c6-f8fde6b105d9",
          "last_message_id": "9017f85d-8cd3-46a8-a88a-2eb7f4c099ea"
      }
  }
  ```

* **gemini**：谷歌gemini pro

  额外参数

  ```
  {
      ...
      "gemini": {
          "type": "api",
          "index": "100001"
      }
  }
  ```

  * index为密钥ID,头部x-auth-id优先级高于index
  * 如果传递Authorization鉴权，还可以传递x-version指定api版本，默认v1beta

  额外返回：

  ```
  {
      ...
      "gemini": {
          "type": "api",
          "index": "100001"
      }
  }
  ```

* **bing**：微软bing chat,有IP要求，不符合会出验证码

  额外参数：

  ```
  {
      ...
      "bing": {
          "conversation": {
              "conversationId": "xxxx",
              "clientId": "xxxx",
              "Signature": "xxx",
              "TraceId": "xxx"
          }
      }
  }
  ```

  * 表示在同一会话基础上进行对话，此值在任一对话通信后会返回

  额外返回：

  ```
  {
      ...
      "bing": {
          "conversationId": "xxxx",
          "clientId": "xxxx",
          "Signature": "xxx",
          "TraceId": "xxx"
          ...
      }
  }
  ```

* **coze**：支持discord和api,走api时model传coze-api

  额外参数

  ```
  {
      ...
      "coze": {
          "conversation": {
              "type": "api",
              "bot_id": "xxxx",
              "conversation_id": "xxx",
              "user": "xxx"
          }
      }
  }
  ```

  * discord只支持一次性对话
  * api通信时，user为标识ID,bot_id为coze app ID,头部x-auth-id对应user,x-bot-id对应bot_id，头部优先鉴权

  额外返回：

  ```
  {
      ...
      "coze": {
          "type": "api",
          "bot_id": "xxxx",
          "conversation_id": "xxx",
          "user": "xxx"
      }
  }
  ```

* **claude**：支持web和api

  额外参数

  ```
  {
      ...
      "claude": {
          "type": "web",
          "index": "100001",
          "conversation": {
              "uuid": "xxxx"
              "model" "xxx",
              ...
          }
      }
  }
  ```

  * 其中，如果没传claude或者传type为api，走claude api接口，其他情况走web
  * index为密钥ID，头部x-auth-id优先级高于index
  * conversation，web专用，表示在同一会话基础上进行对话，此值在任一对话通信后会返回

  额外返回：

  ```
  {
      ...
      "claude": {
          "type": "web",
          "index": "100001",
          "conversation": {
              "uuid": "xxxx"
              "model" "xxx",
              ...
          }
      }
  }
  ```

* **不传或不支持**的provider默认走openai的v1/chat/completions接口

**2. openai相关接口**

* **转发/public-api/\*path**
* **转发/backend-api/\*path**
* **转发/dashboard/\*path**
* **转发/v1/\*path**
* **post /backend-anon/web2api**，web转api
* **post /backend-api/web2api**，web转api

  ```
  通信请求数据如下：
  {
      "action": "next",
      "messages": [{
          "id": "aaa2e4da-d561-458e-b731-e3390c08d8f7",
          "author": {"role": "user"},
          "content": {
              "content_type": "text",
              "parts": ["你是谁？"]
          },
          "metadata": {}
      }],
      "parent_message_id": "aaa18093-c3ec-4528-bb92-750c0f85918f",
      "model": "text-davinci-002-render-sha",
      "timezone_offset_min": -480,
      "history_and_training_disabled": false,
      "conversation_mode": {"kind": "primary_assistant"},
      "websocket_request_id": "bf740f5f-2335-4903-94df-4003819fdade"
  }
  ```

* **post /auth/token/web**，openai chat web登录获取token

  ```
  通信请求数据如下：
  {
      "email": "my@email.com",
      "password": "123456",
      "arkose_token": "xxxx",
      "reset": true
  }
  ```

  * arkose_token，不传自动生成，可能会出验证码，自动解析你需要官网登录后下载har文件放到类似/opt/aiproxy/hars目录下
  * reset，默认不传为false，会根据上次成功获取token保存cookie，根据cookie刷新token，传true重新获取

**3. claude相关接口**

* /claude/web/*path，转发web端，path参数为转发的path，下同
* /claude/api/*path，转发api
* post /claude/api/openai，api转openai api格式，此接口支持头部传递Authorization、x-api-key、x-auth-id鉴权(按此排序依次优先获取)，不传随机获取配置密钥

**4. gemini相关接口**

* /gemini/*path，转发api，path参数为转发的path
* post /gemini/openai，api转openai api格式，此接口支持头部传递Authorization、x-auth-id鉴权(按此排序依次优先获取)，不传随机获取配置密钥
