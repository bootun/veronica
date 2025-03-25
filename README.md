veronica
===
`veronica` 的是**Go项目**的改动影响分析工具。通过在`veronica.yml`中配置您感兴趣的服务，`veronica` 会帮您分析项目的依赖, 并告知您此次的改动可能会产生哪些影响。  
> :construction: 本项目仍处于早期阶段， 可能会经常变动

## 前置条件
 - 已经安装Git
 - 项目使用go module

## 用法
1. 安装veronica
```bash
go install github.com/bootun/veronica@latest
```
2. 在项目的根目录放置[veronica.yaml](./veronica_example.yaml)文件，配置好你关心的函数/方法/变量/常量/结构体
3. 切换至项目目录，并运行veronica:

```bash
cd ${PROJECT_DIR}
veronica impact --old=HEAD~2 --new=HEAD --scope=service
```

该命令会让veronica比较当前指向HEAD的commit与HEAD前两个commit这两份代码之间的差异。  

假设你这两次commit里改动的代码影响到了`veronica.yaml`里其中的三个服务，执行该命令，你会得到类似这样的输出:

```sh
refresh_playlet_info
refresh_playlet_tags
GRPC_BatchGetPlayletInfo
# 如果你的代码让更多的服务受到了影响，veronica会将它输出到这里
```

## 配置文件
veronica在运行时，会在项目跟目录下寻找`veronica.yaml`配置文件, 当前版本的配置文件主要由以下部分组成:  

`version`: 指定veronica配置文件的版本，当前项目还处于早期阶段，变动较大，未来更新可能导致配置文件的语法产生变化，因此使用版本号来进行区分。

```yaml
version: 1.0.0
```

`services`: services下可以定义一系列的service item，通常来说，每个service item都是一个服务，比如CronJob进程、消费者进程、对外提供HTTP或RPC的进程。
> 但services并不局限于服务，它可以是任何你想要让veronica关注的内容(你将会在下面的entrypoint部分来了解它)

```yaml
services: 
  # refresh_playlet_info 是一个CronJob进程
  refresh_playlet_info:
    entrypoint: ...
  # update_playlet 是一个消费者进程
  update_playlet:
    entrypoint: ...
  # playlet_server是一个对外提供RPC服务的进程  
  playlet_server:
    entrypoint: ...
```

`entrypoint`: 每个service都需要有一个entrypoint，entrypoint一般是该服务的入口，它可能是个函数，可能是个方法，可能是个变量。
但entrypoint并不局限于此，它甚至可以是常量/类型声明，只要是你想让veronica关注的内容，都可以写进services里:

```yaml
services: 
  refresh_playlet_info:
    # NewRefreshPlayletInfoCronjob 这个函数是CronJob进程的入口
    entrypoint: 'cmd/cron/refresh_playlet_info.go:NewRefreshPlayletInfoCronjob'

  update_playlet:
    # UpdatePlaylet 是个变量(&cobra.Command)，是消费者进程的入口，通过cobra.AddCommand绑定到root上进行执行
    entrypoint: 'github.com/bootun/some-project/cmd/consumer/update_playlet.go:UpdatePlaylet'

  # or gRPC interface
  # GetPlayletInf是一个gRPC接口实现
  GRPC_GetPlayletInfo:
    # GetPlayletInfo 是一个方法，它的签名如下: 
    # func (svc *PlayletServer) GetPlayletInfo(ctx context.Context, req *pb.GetPlayletInfoReq) (*pb.PlayletBaseInfo, error)
    # 如果你也想关注本次改动对RPC的影响，你可以像这样将RPC接口也纳入veronica的管控
    entrypoint: "internal/server/grpc.go:(*PlayletServer).GetPlayletInfo"    

  # 除此之外，service还支持类型声明(结构体)、常量等顶级字段的依赖分析
  # ...
```
entrypoint的值为你想要关注对象的包路径，比如你的Go module name是`github.com/bootun/some-project`,  
那么你的entrypoint可以是: `github.com/bootun/some-project/cmd/consumer/update_playlet.go:UpdatePlaylet`。  
你也可以使用简略版的相对包名, 例如`cmd/consumer/update_playlet.go:UpdatePlaylet`来表示，veronica会自动为你添加前缀。  

如果你关注的对象是个方法(method), 你需要写出他的receiver，就像上面示例中`GRPC_GetPlayletInfo`那样:
```yaml
GRPC_GetPlayletInfo:
  # func (svc *PlayletServer) GetPlayletInfo(ctx context.Context, req *pb.GetPlayletInfoReq) (*pb.PlayletBaseInfo, error)
  entrypoint: "internal/server/grpc.go:(*PlayletServer).GetPlayletInfo"    
```

## 可配置项

**输出源代码变更可能会产生的全部影响**  

如果你在使用`veronica impact`命令时， 没有加上`--scope`参数，或使用`--scope=all`来指定veronica输出全部的影响时，
veronica会报告改动对整个项目所有的顶层声明产生的影响，包括所有的包级别的函数/方法/结构体/变量/常量，就像下面这样:

```sh
> veronica impact --old HEAD~2 --new HEAD --scope=all

add (*tagRepo).GetAllTagList in github.com/bootun/some-project/infra/mysql/qimao_free/tag_repo.go, dependencies:
  1. github.com/bootun/some-project/internal/app/domain/playlet/playlet_service.go:(*playletService).producePlayletInfo
  2. github.com/bootun/some-project/internal/app/domain/playlet/playlet_service.go:(*playletService).RefreshAllPlayletInfoRds
  ...
modify TagBaseEnt in github.com/bootun/some-project/internal/app/domain/tags/entity/tag_entity.go, dependencies:
  1. github.com/bootun/some-project/internal/app/consumer/update_playlet.go:(*UpdatePlayletConsumer).DealPlaylet
  2. github.com/bootun/some-project/infra/mysql/qimao_free/tag_repo.go:(*tagRepo).GetOneTagById
  3. github.com/bootun/some-project/internal/server/grpc.go:(*PlayletServer).GetPlayletTagSortList
  ...
  18. github.com/bootun/some-project/infra/mysql/qimao_free/tag_repo.go:(*tagRepo).GetAllTagList
remove (*playletService).setPlayletCacheInfo in github.com/bootun/some-project/internal/app/domain/playlet/playlet_service.go, dependencies:
  1. github.com/bootun/some-project/internal/app/consumer/update_playlet.go:(*UpdatePlayletConsumer).BatchWrite
  ...
  8. github.com/bootun/some-project/internal/app/consumer/update_playlet.go:(*UpdatePlayletConsumer).DealPlaylet
```
该命令会详细告诉你对那些内容做了哪些操作(add/modify/remove), 并报告该修改产生的影响。

## 未来规划
1. 当前GRPC这种方式对超多接口的项目来说，需要配置非常多的service，veronica计划改进这一点。
2. 当前veronica只能分析go文件带来的影响，接下来我计划实现service的`hooks`和`ignores`字段，使任意文件的改动都能与service进行关联。
3. veronica输出变更产生的影响时，计划增加对Go模版语法的支持。

## 命名背景
`Veronica`取自钢铁侠的同名外太空支援系统，在你需要升级战甲时，只需要通知维罗妮卡，它就会将战甲的模块从外太空发送给你，重新组合后完成升级。

## 相关阅读
 - [基于大仓库的微服务差异化构建工具](https://mp.weixin.qq.com/s/XQqDyJyh1u6jU0PmUdS0LA)