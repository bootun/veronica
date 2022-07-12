Veronica
===
`Veronica` 的目标是成为**Go项目**的差异化构建指导工具。试想一下，如果你的项目分为许多微服务，而这个项目是以[Monorepo](https://en.wikipedia.org/wiki/Monorepo)的形式组织的，那么每次构建时，因为无法知道修改的文件会影响哪些服务，因此必须要构建所有的服务。`Veronica` 就是为了解决这一问题而诞生的，给定一个或多个文件，`Veronica` 会帮您分析项目的依赖, 并告知您该文件可能会产生哪些影响。  
> :construction: 本项目仍处于早期阶段， 可能会经常变动

## 前置条件
 - Git
 - 项目使用go module

## 用法
1. 在项目的根目录放置[veronica.yaml](./veronica_example.yaml)文件
2. 切换至项目目录，输入以下代码:
```bash
git log --name-only -1 --pretty=format:"" | veronica
```
**详细输出效果:**  
<details>
<pre>
改动了 pkg/apigateway/spec 包中的 pkg/apigateway/spec/api.swagger.json 文件,可能会影响这些包的构建:
    - cmd/api-gateway

改动了 pkg/apigateway/spec 包中的 pkg/apigateway/spec/static.go 文件,可能会影响这些包的构建:
    - cmd/api-gateway

改动了 pkg/pb 包中的 pkg/pb/merchant_assets.pb.go 文件,可能会影响这些包的构建:
    - cmd/api-gateway
    - cmd/assets-cron
    - cmd/currency-cron
    - cmd/iam-cron
    - cmd/iam-manager
    - cmd/across-cron
    - cmd/assets-manager
    - cmd/currency-manager
    - cmd/system-cron
    - cmd/system-manager
    - cmd/across-manager

改动了 pkg/pb 包中的 pkg/pb/merchant_assets.pb.gw.go 文件,可能会影响这些包的构建:
    - cmd/api-gateway
    - cmd/assets-cron
    - cmd/currency-cron
    - cmd/iam-cron
    - cmd/iam-manager
    - cmd/across-cron
    - cmd/assets-manager
    - cmd/currency-manager
    - cmd/system-cron
    - cmd/system-manager
    - cmd/across-manager

改动了 pkg/service/assets 包中的 pkg/service/assets/handler_merchant_assets.go 文件,可能会影响这些包的构建:
    - cmd/assets-manager
</pre>
</details>

**简略输出效果：**  
<details>
<pre>
cmd/api-gateway
cmd/across-cron
cmd/currency-cron
cmd/iam-manager
cmd/system-cron
cmd/system-manager
cmd/across-manager
cmd/assets-cron
cmd/assets-manager
cmd/currency-manager
cmd/iam-cron
</pre>
</details>

> 在0.0.2版本之后，默认输出精简模式，想要获取详细输出，需要在`veronica.go`中调用`project.ReportImpact()`并自行编译。目前我正在积极探索更加灵活的输出方式。

## 已实现功能
 - 解析所有文件/目录之间的依赖关系
 - 报告可能影响构建的包

## 命名背景
`Veronica`取自钢铁侠的同名外太空支援系统，在你需要升级战甲时，只需要通知维罗妮卡，它就会将战甲的模块从外太空发送给你，重新组合后完成升级。

## 未来规划
 - 支持配置文件灵活配置
 - 成为CI/CD的一部分， 根据git commit中改动的文件， 结合配置文件，告知您哪些服务受影响，需要重新构建(已完成,但输出格式需优化)
