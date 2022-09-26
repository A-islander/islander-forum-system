# 岛民论坛系统
这里是岛民论坛系统，提供查看串，发串，回串，sage服务。  
采用微服务架构，与岛民用户系统通过rpc通信，获取用户信息。  

## github actions
本服务系统于2022/09/27使用github actions构建，若需要使用可自行修改.github/workflows下的deployment.yml

## 预计目标
1. 串查看（完成）
2. 板块查看（完成）
3. 发/回串（完成）
4. Sage系统（阶段开发完毕，迭代中）

## 接口文档
通过apiPost生成的接口文档[地址](https://docs.apipost.cn/preview/b58077f3ebc9caeb/6a197cc600cf6f5c)

## 配置文件
```json
{
    "userName": "", // 数据库用户名
    "passWord": "", // 数据库密码
    "ip": "",       // 数据库地址
    "database": "", // 数据库名称
    "sageNum": 20,  // sage差值
    "rpcIp": "",    // userServer的地址
    "redisIp": "",  // redis地址
    "buffTime": 0,  // redis数据缓存时间，单位秒
}
```