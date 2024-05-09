AI Chat集成转发、代理
==========

golang版ai chat工具集成，目前支持openai、bing、claude、gemini和coze，后续陆续增加

# 使用

**1. 源码构造**

```
git clone https://github.com/zatxm/any-proxy
cd any-proxy
go build -ldflags "-s -w" -o anyproxy cmd/main.go
./anyproxy -c /whereis/c.yaml
```

**2. docker**

* 本地构建

```
git clone https://github.com/zatxm/any-proxy
cd any-proxy
docker build -t zatxm120/myproxy . # 镜像名称可以自行定义
```

* docker库

```
docker pull zatxm120/myproxy
```

* 启用

```
docker run -d --name myproxy --restart --net=host always zatxm120/myproxy #自身网络，默认配置端口8999
docker run -d --name myproxy --restart always -p 8084:8999 zatxm120/myproxy #映射端口，默认8999,实际还是要看配置文件的端口号
docker run -d --name myproxy --restart always -p 8084:8999 -v /your-app-data:/anp/data zatxm120/myproxy #映射文件夹，包含配置文件等
```

## 配置说明
**1. 创建数据目录**

假设放在/opt/anyproxy目录上，目录根据自己需求创建

```
mkdir -p /opt/anyproxy/ssl #放置https证书
mkdir -p /opt/anyproxy/hars #放置openai登录har
mkdir -p /opt/anyproxy/pics #放置验证码，一般没用到
mkdir -p /opt/anyproxy/etc #配置文件目录，配置文件复制到该目录
```
**2. 配置文件c.yaml**

源码中的etc/c.yaml为配置实例，复制到/opt/anyproxy/etc目录下修改

**3. docker映射目录**

```
docker run -d --name myproxy --restart always -p 8084:8999 -v /opt/anyproxy:/anp/data zatxm120/myproxy
```

## 接口应用说明

**1. 通用接口/c/v1/chat/completions**

支持通信头部header加入密钥(不加随机获取配置文件的密钥)，一般以下两种：

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
"openai": {
    "conversation": {
        "conversation_id":"697b28e8-e228-4abb-b356-c8ccdccf82f3",
        "parent_message_id":"dd6d9561-bebe-42da-91c6-f8fde6b105d9",
        "last_message_id":"9017f85d-8cd3-46a8-a88a-2eb7f4c099ea"
    }
}
```

表示在同一会话基础上进行对话，此值在任一对话通信后会返回

* **gemini**：谷歌gemini pro
* **bing**：微软bing chat,有IP要求，不符合会出验证码
* **coze**：支持discord和api,走api时model传coze-api
* **claude**：目前支持claude web chat,后续加入api to api

额外参数

```
"claude": {
    "type": "web",
    "index": "100001",
    "conversation": {
        "uuid": "xxxx"
        "model" "xxx",
        ...
    }
}
```

* 其中，如果没传claude或者传type为api，走claude api接口，其他情况走web
* index为密钥ID,优先级高于x-auth-id
* conversation，web专用，表示在同一会话基础上进行对话，此值在任一对话通信后会返回

```
"openai": {
    "conversation": {
        "conversation_id":"697b28e8-e228-4abb-b356-c8ccdccf82f3",
        "parent_message_id":"dd6d9561-bebe-42da-91c6-f8fde6b105d9",
        "last_message_id":"9017f85d-8cd3-46a8-a88a-2eb7f4c099ea"
    }
}
```

* **不传或不支持**的provider默认走openai的v1/chat/completions接口

**2. openai web转api**

* post /backend-anon/web2api
* post /backend-api/web2api

```
通信请求数据如下：
{
    "action":"next",
    "messages":[{
        "id":"aaa2e4da-d561-458e-b731-e3390c08d8f7",
        "author":{"role":"user"},
        "content":{
            "content_type":"text",
            "parts":["你是谁？"]
        },
        "metadata":{}
    }],
    "parent_message_id":"aaa18093-c3ec-4528-bb92-750c0f85918f",
    "model":"text-davinci-002-render-sha",
    "timezone_offset_min":-480,
    "history_and_training_disabled":false,
    "conversation_mode":{"kind":"primary_assistant"},
    "websocket_request_id":"bf740f5f-2335-4903-94df-4003819fdade"
}
```

**3. claude相关接口**

* /claude/web/*path，转发web端，path参数为转发的path，下同
* /claude/api/*path，转发api
* post /claude/api/openai，api转openai api格式，此接口支持头部传递Authorization、x-api-key、x-auth-id鉴权(按此排序依次优先获取)，不传随机获取配置密钥
