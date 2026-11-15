# Orbiqo 部署说明

基于 Sub2API 改品牌后的自有部署。服务器：`root@osaka`（1GB 内存的共享机器，容器已限内存）。

## 目录（服务器 `/opt/orbiqo`）

- `docker-compose.yml`：app 200M、postgres 160M、redis 48M 内存上限，只监听 `127.0.0.1:18080`
- `.env`：密钥和连接池参数（不入库，权限 600）
- `apply-brand.sh` + `home.html`：把站名、副标题、自定义首页写入设置表并重启 app

## 构建镜像（本机）

Dockerfile 里的 `corepack` 走不了 HTTP 代理，用临时副本构建，不改仓库里的文件：

    sed 's|^RUN corepack enable \&\& corepack prepare pnpm@9 --activate|RUN NODE_USE_ENV_PROXY=1 corepack enable \&\& NODE_USE_ENV_PROXY=1 corepack prepare pnpm@9 --activate|' Dockerfile > /tmp/Dockerfile.proxy
    docker build -f /tmp/Dockerfile.proxy \
      --build-arg HTTP_PROXY=$PROXY --build-arg HTTPS_PROXY=$PROXY --build-arg NO_PROXY=localhost,127.0.0.1 \
      --build-arg GOPROXY=https://proxy.golang.org,direct --build-arg GOSUMDB=sum.golang.org \
      -t orbiqo:dev .

## 更新上线

    docker save orbiqo:dev | gzip -1 | ssh root@osaka 'gunzip | docker load'
    ssh root@osaka 'cd /opt/orbiqo && docker compose -p orbiqo up -d'

## zcode-provider 额度

Anthropic API Key 账号如果模型映射使用 `c/` 或 `s/` 短别名，或凭证里 `quota_source` 为 `zcode`，账号列表会请求该网关的 `GET /v1/orbiqo/quota`，并在用量列显示 Coding Plan 的 5 小时和每周窗口。

## nginx / 证书

- 站点配置：`nginx-orbiqo.cc.conf` → `/etc/nginx/sites-enabled/orbiqo.cc.conf`
- 证书：`acme.sh`（Let's Encrypt，ECC），装到 `/etc/nginx/ssl/orbiqo.cc.{crt,key}`，续期后自动 reload nginx
- 同机还有 9router、x-ui 等站点，不要改 http 级别的全局指令
