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
docker run -d --name myproxy --restart always zatxm120/myproxy
docker run -d --name myproxy --restart always -p 8084:8999 zatxm120/myproxy #映射端口，默认8999,实际还是要看配置文件的端口号
docker run -d --name myproxy --restart always -p 8084:8999 -v /anp/data:/your-app-data zatxm120/myproxy #映射文件夹，包含配置文件等
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
docker run -d --name myproxy --restart always -p 8084:8999 -v /anp/data:/opt/anyproxy zatxm120/myproxy
```
