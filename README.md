# lockstepserver
golang版帧同步服务器

[![GoDoc](https://godoc.org/github.com/bailu1901/lockstepserver?status.png)](https://godoc.org/github.com/bailu1901/lockstepserver)
[![Build Status](https://travis-ci.org/bailu1901/lockstepserver.svg?branch=master)](https://travis-ci.org/bailu1901/lockstepserver)
[![Go Report](https://goreportcard.com/badge/github.com/bailu1901/lockstepserver)](https://goreportcard.com/report/github.com/bailu1901/lockstepserver)
---
		
`帧同步服务器目标是作为一个可以在运行时扩展，完全脱离玩法逻辑的帧同步服务器。`
* 采用KCP(可根据需求改成其他协议)作为网络底层
* 帧同步作为同步方式
* etcd做为集群注册发现
* protobuf作为传输协议
* 服务器间传输可以用grpc和http

---

### 创建房间  

1. http方式  
	1. pvp启动时参数 -web=10002
	1. 浏览器打开 http://127.0.0.1:10002
	1. Room是房间ID，Member填参战者ID(用,隔开)

---

### 流程描述([proto文件](pb/message.proto))  

* 初始化网络层，目前使用的[kcp](https://github.com/skywind3000/kcp)
* 消息包格式
	```
	|-----------------------------message-----------------------------------------|
	|----------------------Header------------------|------------Body--------------|
	|------Body Length-------|--------Msg ID-------|------------Body--------------|
	|---------uint16---------|---------uint8-------|------------bytes-------------|
	|-----------2------------|----------1----------|-----------len(Body)----------|
	```
* 消息流程  
	1. 客户端发送第一个连接的消息包  
		C->S: `MSG_Connect & C2S_ConnectMsg`
	1. 服务端给返回连接结果  
		S->C: `MSG_Connect & S2C_ConnectMsg`
	1. 如果2返回ok，客户端向服务端发送进入房间消息  
		C->S: `MSG_JoinRoom`
	1. 服务端返回进入房间消息  
		S->C: `MSG_Connect & S2C_JoinRoomMsg`
	1. 客户端这时进入读条，并广播读条进度，其他客户端收到广播读条进度  
		C->S: `MSG_Progress & C2S_ProgressMsg`  
		S->C: `MSG_Progress & S2C_ProgressMsg`  **注：广播者收不到这个消息**
	1. 客户端告诉服务端自己已经准备好  
		C->S: `MSG_Ready`  
		S->C: `MSG_Ready`  
	1. 当所有客户端都已经准备好，服务端广播开始  
		S->C: `MSG_Start`  
	1. 客户端可以进入游戏状态，客户端不停的向服务端发送操作，服务端不停的广播帧数据  
		∞ C->S: `MSG_Input & C2S_InputMsg`  
		∞ S->C: `MSG_Frame & S2C_FrameMsg`  
	1. 当客户端游戏逻辑结束告诉服务端自己结束  
		C->S: `MSG_Result & C2S_ResultMsg`  
		S->C: `MSG_Result`  
	1. 当客户端收到`MSG_Result`或者`MSG_Close`客户端断开网络连接进入其他流程  
		**注：客户端收到MSG_Result表示服务端已经收到并处理的客户端发来的结果**  
		**注：客户端收到MSG_Close表示服务端房间已经关闭，客户端如果游戏流程没完也要强制退出**

---

### 断线重连

* 客户端只要发 C->S: `MSG_Connect & C2S_ConnectMsg` **(前提是当前游戏房间还存在)**即可进入房间，服务端会把之前的帧分批次发给客户端。(这里可以考虑改成客户端请求缺失的帧)
	



