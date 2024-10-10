# operator-sdk-demo

安装operator-sdk
使用
```shell
git clone https://github.com/operator-framework/operator-sdk
cd operator-sdk
git checkout master
make install
```

创建项目

```shell
operator-sdk new operator-sdk-demo --repo github.com/chenliujin/operator-sdk-demo
cd operator-sdk-demo
```

创建API

```shell
operator-sdk create api --group ${group_naem} --version v1 --kind ${kind_naem} --resource --controller    
```

构建镜像

```shell
operator-sdk build chenliujin/operator-sdk-demo
```
