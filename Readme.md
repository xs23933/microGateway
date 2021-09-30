# micro gateway 

* api register
* manage api
* save api

http://{mgw.addr}/mgw api list



## server name

### name default "" use server address

name string

## health check  Optional

### check method
check.method string get | post | options | put | delete

### check path 
check.path string /check or you path, default "" if not check

### interval unit second
check.interval int check interval , default 30 second

### timeout unit second
check.timeout int check timeout, default 30 second

## server configure
{host}: [{api}, {api}]

## demo


* {mgw.addr} this server online address
* {microservice.ip} you micro service address


POST http://{mgw.addr}/mgw/sign

Payload:

```json
{
  "name": "account microserver by jack",
  "check": {
    "method": "get",
    "path": "/check",
    "interval": 30,
    "timeout": 50
  },
  "http://microservice.ip:40000": [
    "/api/users/auth",
    "/api/users/authorize",
    "/api/users/department",
    "/api/users/department/*",
    "/api/users/department/sync",
    "/api/users/dept/us/*",
    "/api/users/sync/*"
  ]
}
```