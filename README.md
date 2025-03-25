# veronica

[![Go Report Card](https://goreportcard.com/badge/github.com/bootun/veronica)](https://goreportcard.com/report/github.com/bootun/veronica)

`veronica` 是一个 **Go项目** 的改动影响分析工具。通过在 `veronica.yml` 中配置您感兴趣的服务，`veronica` 会帮您分析项目的依赖，并告知您此次的改动可能会产生哪些影响。

veronica通常可以用在:
 - 基于大仓的微服务CI/CD自动化流程中，帮助你分析此次代码提交产生的改动，只构建受影响的服务
 - 开发/测试人员作为参考，确定代码变更产生的影响范围，避免遗漏
 - AI Code Review时作为上下文提供给AI，让AI产生更加精确的输出

> :construction: 本项目仍处于早期阶段，可能会经常变动

## 目录
- [前置条件](#前置条件)
- [用法](#用法)
- [配置文件](#配置文件)
  - [version](#version)
  - [services](#services)
  - [entrypoint](#entrypoint)
- [可配置项](#可配置项)
- [未来规划](#未来规划)
- [命名背景](#命名背景)
- [相关阅读](#相关阅读)

## 前置条件
- 已经安装 Git
- 项目使用 go module

## 用法

1. 安装 veronica
```bash
go install github.com/bootun/veronica@latest
```

2. 在项目的根目录放置 [veronica.yaml](./veronica_example.yaml) 文件，配置好你关心的函数/方法/变量/常量/结构体

3. 切换至项目目录，并运行 veronica:
```bash
cd ${PROJECT_DIR}
veronica impact --old=HEAD~2 --new=HEAD --scope=service
```

该命令会让 veronica 比较当前指向 HEAD 的 commit 与 HEAD 前两个 commit 这两份代码之间的差异。

假设你有N个服务，你这两次 commit 里改动的代码影响到了 `veronica.yaml` 里其中的三个服务，执行该命令，你会得到类似这样的输出：

```sh
refresh_playlet_info
refresh_playlet_tags
GRPC_BatchGetPlayletInfo
# 如果你的代码让更多的服务受到了影响，veronica 会将它输出到这里
```

## 配置文件

veronica 在运行时，会在项目根目录下寻找 `veronica.yaml` 配置文件。当前版本的配置文件主要由以下部分组成：

### version

指定 veronica 配置文件的版本。当前项目还处于早期阶段，变动较大，未来更新可能导致配置文件的语法产生变化，因此使用版本号来进行区分。

```yaml
version: 1.0.0
```

### services

services 下可以定义一系列的 service item，通常来说，每个 service item 都是一个服务，比如 CronJob 进程、消费者进程、对外提供 HTTP 或 RPC 的进程。

> 但 services 并不局限于服务，它可以是任何你想要让 veronica 关注的内容（你将会在下面的 entrypoint 部分来了解它）

```yaml
services: 
  # refresh_playlet_info 是一个 CronJob 进程
  refresh_playlet_info:
    entrypoint: ...
  # update_playlet 是一个消费者进程
  update_playlet:
    entrypoint: ...
  # playlet_server 是一个对外提供 RPC 服务的进程  
  playlet_server:
    entrypoint: ...
```

### entrypoint

每个 service 都需要有一个 entrypoint，entrypoint 一般是该服务的入口，它可能是个函数，可能是个方法，可能是个变量。
但 entrypoint 并不局限于此，它甚至可以是常量/类型声明，只要是你想让 veronica 关注的内容，都可以写进 services 里：

```yaml
services: 
  refresh_playlet_info:
    # NewRefreshPlayletInfoCronjob 这个函数是 CronJob 进程的入口
    entrypoint: 'cmd/cron/refresh_playlet_info.go:NewRefreshPlayletInfoCronjob'

  update_playlet:
    # UpdatePlaylet 是个变量(&cobra.Command)，是消费者进程的入口，通过 cobra.AddCommand 绑定到 root 上进行执行
    entrypoint: 'github.com/bootun/some-project/cmd/consumer/update_playlet.go:UpdatePlaylet'

  # gRPC interface
  # GetPlayletInfo 是一个 gRPC 接口实现
  GRPC_GetPlayletInfo:
    # GetPlayletInfo 是一个方法，它的签名如下: 
    # func (svc *PlayletServer) GetPlayletInfo(ctx context.Context, req *pb.GetPlayletInfoReq) (*pb.PlayletBaseInfo, error)
    entrypoint: "internal/server/grpc.go:(*PlayletServer).GetPlayletInfo"    
```

entrypoint 的值为你想要关注对象的包路径。  
比如你的 Go module name 是 `github.com/bootun/some-project`，
那么你的 entrypoint 可能是：  
`github.com/bootun/some-project/cmd/consumer/update_playlet.go:UpdatePlaylet`

你也可以使用更简短的相对包名来表示，例如`cmd/consumer/update_playlet.go:UpdatePlaylet`，veronica 会自动为你添加前缀。

如果你关注的对象是个方法（method），你需要写出他的 receiver，就像上面示例中 `GRPC_GetPlayletInfo` 那样：
```yaml
GRPC_GetPlayletInfo:
  # func (svc *PlayletServer) GetPlayletInfo(ctx context.Context, req *pb.GetPlayletInfoReq) (*pb.PlayletBaseInfo, error)
  entrypoint: "internal/server/grpc.go:(*PlayletServer).GetPlayletInfo"    
```

## 可配置项

**输出源代码变更可能会产生的全部影响**

如果你在使用 `veronica impact` 命令时，没有加上 `--scope` 参数，或使用 `--scope=all` 来指定 veronica 输出全部的影响时，
veronica 会报告改动对整个项目所有的顶层声明产生的影响，包括所有的包级别的函数/方法/结构体/变量/常量，就像下面这样：

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

该命令会详细告诉你对哪些内容做了哪些操作（add/modify/remove），并报告该修改产生的影响。

## 未来规划

1. 当前 GRPC 这种方式对超多接口的项目来说，需要配置非常多的 service，veronica 计划改进这一点
2. 当前 veronica 只能分析 go 文件带来的影响，接下来我计划实现 service 的 `hooks` 和 `ignores` 字段，使任意文件的改动都能与 service 进行关联
3. veronica 输出变更产生的影响时，计划增加对 Go 模版语法的支持

## 命名背景

`Veronica` 取自钢铁侠的同名外太空支援系统，在你需要升级战甲时，只需要通知维罗妮卡，它就会将战甲的模块从外太空发送给你，重新组合后完成升级。

## 相关阅读

- [基于大仓库的微服务差异化构建工具](https://mp.weixin.qq.com/s/XQqDyJyh1u6jU0PmUdS0LA)