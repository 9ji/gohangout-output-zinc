# gohangout zinc输出插件
该项目为gohangout的zinc output插件，用于将消息写入zinc搜索引擎。 

gohangout: [https://github.com/childe/gohangout](https://github.com/childe/gohangout)  
zinc: [https://github.com/prabhatsharma/zinc](https://github.com/prabhatsharma/zinc)  
## 使用方法
### 1. 编译

将本项目中的`zinc_output.go`文件拷贝到gohangout工程下，然后执行如下编译命令
```bigquery
CGO_ENABLED=1 go build -buildmode=plugin -o zinc_output.so zinc_output.go
```

需要注意：
1. 由于golang plugin的局限性，要求插件和主项目的依赖保持一致，所以最好的做法是将plugin源码拷贝到gohangout工程中进行编译；
2. 由于golang plugin兼容性的问题，跨系统交叉编译会存在问题，所以编译过程务必在gohangout运行的目标系统上执行；
3. gohangout官方[release](https://github.com/childe/gohangout/releases) 的可执行程序，不支持运行外挂插件，请clone gohangout项目代码自行编译，编译需要启用`CGO_ENABLED=1`
   否则无法加载plugin，也务必在gohangout运行的目标系统上执行编译操作，比如：
   ```bigquery
    CGO_ENABLED=1 go build
   ```
### 2. 使用
plugin编译好后，将so文件拷贝到gohangout运行目录，在配置文件中指定plugin的绝对路径即可，配置参照下节。

### 3. 配置
```bigquery
outputs:
    - /opt/gohangout/zinc_output.so:
        addresses:
          - http://10.1.250.157:4080
          - http://10.1.250.158:4080
        index: '%{appName}-%{+2006-01-02}'
        username: admin
        password: adminPwd
        batch_size: 100
        batch_flush_interval: 5
        concurrency: 4
```
1. `/opt/gohangout/zinc_output.so`为plugin绝对路径；
2. `addresses`为zinc的api接口路径，可以填写多个，目前负载均衡为简单随机；
3. `index` zinc的索引规则；
4. `username` zinc API接口鉴权用户名；
5. `password` zinc AIP接口鉴权密码；
6. `batch_size` 每次批量写入的消息数量；
7. `batch_flush_interval` 批量写入刷新间隔，单位s；
8. `concurrency` 批量写入并发度，默认为4；
