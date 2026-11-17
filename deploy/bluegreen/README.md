# Sub2API 蓝绿发布

适用当前生产配置：域名 `oapi.yundata.ai`，Compose 应用服务
`sub2api-foreign`，现有蓝槽端口 `8081`，绿槽端口 `8082`。
需要 Linux、Python 3.6+、Docker Compose v2、`ss` 和已运行的 Nginx。

应用代码、业务协议和原有计费规则不变。两个应用共用原有
`postgres-foreign`、`redis-foreign`、`foreign_data`、JWT/TOTP 配置，
不创建第二套数据库。新槽端口仅绑定 `127.0.0.1`。

## 首次接入生产 Nginx

先将 `release.py` 和 `oapi-upstream.conf` 放到现有 Compose 目录的
`bluegreen/` 下。以下命令均在该 Compose 目录执行。

1. 检查 `8082` 未被其他服务占用，备份 Nginx 主配置到 `back/`：

   ```sh
   mkdir -p back
   cp -a /etc/nginx/nginx.conf "back/nginx.conf.$(date +%Y%m%d-%H%M%S)"
   cp bluegreen/oapi-upstream.conf /etc/nginx/conf.d/oapi-bluegreen.conf
   ```

2. 在原配置 **`oapi.yundata.ai` 的 HTTPS server 块内**，将两处
   `proxy_pass http://127.0.0.1:8081;` 都改为：

   ```nginx
   proxy_pass http://oapi_sub2api;
   ```

   两处分别是静态资源 location 和 `location /`。
   主配置原有 `http { include /etc/nginx/conf.d/*.conf; }` 会加载 upstream。
   保留证书、`/images/`、`/doc/`、日志、其他域名及现有超时设置。
   不要同时保留另一份重复的 `upstream oapi_sub2api`。

3. 检查并平滑加载，仍指向现有蓝槽，不重启应用：

   ```sh
   nginx -t && nginx -s reload
   sudo env DOCKER_CONFIG="$HOME/.docker" python3 bluegreen/release.py init \
     --upstream-file /etc/nginx/conf.d/oapi-bluegreen.conf
   ```

   若 Compose 文件不是 `docker-compose.yml`，通过 `init --compose-file` 指定。
   若 Nginx 在容器内，用 `--nginx-container NAME`；upstream 文件必须以**目录挂载**
   到该容器的 `conf.d` 下，保证原子替换后容器看得到新文件。

## 日常发布

完成代码验证，构建并推送唯一的 `:last` 镜像后：

```sh
sudo env DOCKER_CONFIG="$HOME/.docker" python3 bluegreen/release.py deploy
sudo env DOCKER_CONFIG="$HOME/.docker" python3 bluegreen/release.py status
```

脚本拉取 `:last`，解析并按本地不可变 image ID 启动空闲槽，
不会因可变标签而重建正在接客的槽。Compose 配置从原文件和 `.env` 读取，
生成的应用专用配置仅允许文件所有者读取，存于 `.bluegreen/`。
数据库、Redis、原 Compose 文件不会被重建或改写。
通过 `sudo` 执行且未显式指定 `DOCKER_CONFIG` 时，脚本使用执行用户已有的
Docker 登录配置，不复制凭据，避免 root 未登录仓库导致拉取失败。

候选应用必须通过 Docker 健康检查、JSON `/health`、管理员 API 的
JSON 401 检查，才替换 upstream 文件并执行 `nginx -t` / 平滑 reload。
切换后的公网入口也会进行相同检查；失败则恢复旧 upstream，
保留两套应用供排查。发布锁会阻止同时操作。

旧 Nginx worker 全部退出、旧应用端口无存量 TCP 连接后才停止旧应用。
脚本还检查共享 Redis 中的异步图片任务；存在 `processing` 任务时继续保留旧槽，
避免 HTTP 已返回任务 ID、后台仍在生图时进程被停止。该检查有意保守，
新槽创建的异步图片任务同样会延后旧槽回收。
默认等待 120 秒；超过等待时间会保留旧应用，而不会强杀长请求。

```sh
sudo env DOCKER_CONFIG="$HOME/.docker" python3 bluegreen/release.py finish
```

旧槽未排空时禁止下次覆盖它。其他域名的长连接也可能使 Nginx 旧 worker
长期存在，这是保守保护；不要为发布强杀它们。
客户端必须通过 Nginx 域名访问，直接访问旧应用端口无法自动切换。

## 回退与中断恢复

```sh
sudo env DOCKER_CONFIG="$HOME/.docker" python3 bluegreen/release.py rollback
```

恢复上一槽的原 image ID，不重新拉取旧的可变标签。
只有数据库仍兼容旧版本时才能回退。
若进程在切换中途退出，脚本会保留恢复日志并禁止其他修改：

```sh
sudo env DOCKER_CONFIG="$HOME/.docker" python3 bluegreen/release.py recover
sudo env DOCKER_CONFIG="$HOME/.docker" python3 bluegreen/release.py finish
```

配置、状态与旧镜像信息的备份均放在 `back/bluegreen-时间/`。
不要在蓝绿运行期间执行原来的 `docker compose up -d` 更新应用，
也不要删除 `.bluegreen/`、执行 `compose down` 或清理回退镜像。
数据库迁移发布仍需单独备份数据库。新增迁移必须兼容同时运行的旧应用；
删除字段、改字段含义等不兼容迁移需要专门维护窗口，蓝绿不能消除这种风险。

## 后台任务约束

图片/视频结算已通过数据库行锁、租约和记账幂等保护多实例。
定时模型测试和渠道主动监控目前只有进程内协调，可能重复调用上游。
脚本在初始化和发布前读取数据库；若存在已启用的测试计划或渠道监控，
要求先在后台暂停，完成排空后再恢复。它不会自行修改这些业务配置。
其他只读余额探测可能在短暂并行期各执行一次。
不要在发布过程中重新启用这些任务。

两个槽共享数据目录时，生成的 Compose 使用 SELinux 的共享 `:z`
标签代替独占 `:Z`，避免第二个容器重标记目录使第一个容器失去访问权限。
对已有目录的权限和配置应在首次接入前核对。

本目录脚本只负责部署，不会自动构建应用；镜像必须始终来自完整定制源码，
构建时使用 `--provenance=false --sbom=false`。

## 验证记录

2026-11-17 在测试服务器使用已有应用镜像完成验证：

- 15 项脚本测试通过，覆盖配置保留、候选健康失败、切流失败恢复、
  发布锁、后台任务检查、长连接与异步任务排空保护。
- 使用独立的本地测试入口 `18088`、蓝绿端口 `18081/18082`，
  完成蓝槽到绿槽切换、长请求排空、旧槽停止以及回滚。
- 演练期间 401 次 `/health` 检查全部成功；跨越切流的模拟长请求返回 HTTP 200。
- 仅使用健康检查、无认证管理员请求及模拟长请求，未创建收费图片或视频任务。

这验证了同一现有镜像的发布机制，没有验证未来版本的数据库迁移兼容性，
也没有修改生产 Nginx 或生产应用。生产首次接入仍需按上面的步骤执行。
