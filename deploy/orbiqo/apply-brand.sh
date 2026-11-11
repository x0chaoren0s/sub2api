#!/usr/bin/env bash
# 把 Orbiqo 的站名、副标题、自定义首页写入设置表（幂等，可重复执行）。
# 用法：在 compose 所在目录执行  bash apply-brand.sh
# 依赖：当前目录有 .env（读取数据库账号），容器名 orbiqo-db / orbiqo-app。
set -euo pipefail
cd "$(dirname "$0")" && HERE=$PWD
cd "${DEPLOY_DIR:-$PWD}"
set -a; . ./.env; set +a

SITE_NAME="${SITE_NAME:-Orbiqo}"
SITE_SUBTITLE="${SITE_SUBTITLE:-AI API 接入服务}"
HOME_HTML="$(cat "$HERE/home.html")"

docker exec -i -e PGPASSWORD="$POSTGRES_PASSWORD" orbiqo-db \
  psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" \
  -v name="$SITE_NAME" -v sub="$SITE_SUBTITLE" -v home="$HOME_HTML" <<'SQL'
INSERT INTO settings (key, value, updated_at) VALUES
  ('site_name', :'name', now()),
  ('site_subtitle', :'sub', now()),
  ('home_content', :'home', now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();
SQL

# 公共设置在启动时注入页面，重启后才生效
docker restart orbiqo-app >/dev/null
echo "brand applied; orbiqo-app restarted"
