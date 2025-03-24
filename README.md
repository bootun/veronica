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



## 命名背景
`Veronica`取自钢铁侠的同名外太空支援系统，在你需要升级战甲时，只需要通知维罗妮卡，它就会将战甲的模块从外太空发送给你，重新组合后完成升级。

## 相关阅读
 - [基于大仓库的微服务差异化构建工具](https://mp.weixin.qq.com/s/XQqDyJyh1u6jU0PmUdS0LA)