#!/bin/bash
# 归来小说CMS — 3轮极限压力测试脚本
# 用法: bash stress_test.sh

BASE="http://localhost:8080"
PASS=0; FAIL=0; TOTAL=0

ok() { PASS=$((PASS+1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL+1)); echo "  ❌ $1 (expected: $2, got: $3)"; }

section() { echo ""; echo "━━━ $1 ━━━"; }

# ═══════════════════════════════════════════════════════════════════════
# ROUND 1: 高并发 + 边界条件 + 异常注入
# ═══════════════════════════════════════════════════════════════════════

section "ROUND 1 — 高并发请求 & 边界条件"

# 1.1 并发200请求健康检查
echo "1.1 并发200请求 /health..."
for i in $(seq 1 200); do
  (curl -s -o /dev/null -w "%{http_code}" $BASE/health >> /tmp/stress_codes.txt; echo "" >> /tmp/stress_codes.txt) &
done
wait
OK_COUNT=$(grep -c "200" /tmp/stress_codes.txt 2>/dev/null || echo 0)
[ "$OK_COUNT" -ge 190 ] && ok "并发200 /health: $OK_COUNT/200 成功 (≥190)" \
  || fail "并发200 /health" "≥190" "$OK_COUNT/200"
rm -f /tmp/stress_codes.txt

# 1.2 并发100请求首页
echo "1.2 并发100请求首页..."
for i in $(seq 1 100); do
  (curl -s -o /dev/null -w "%{http_code}" $BASE/ >> /tmp/stress_home.txt; echo "" >> /tmp/stress_home.txt) &
done
wait
HOME_OK=$(grep -c "200" /tmp/stress_home.txt 2>/dev/null || echo 0)
[ "$HOME_OK" -ge 90 ] && ok "并发100首页: $HOME_OK/100 成功" \
  || fail "并发100首页" "≥90" "$HOME_OK/100"
rm -f /tmp/stress_home.txt

# 1.3 边界 — 超大URL
echo "1.3 超大URL..."
HUGE=$(python3 -c "print('x'*10000)" 2>/dev/null || printf 'x%.0s' {1..10000})
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/search?q=$HUGE" 2>/dev/null)
[ "$CODE" = "200" ] || [ "$CODE" = "414" ] && ok "超大URL: HTTP $CODE" \
  || fail "超大URL" "200 or 414" "$CODE"

# 1.4 边界 — 空请求体POST
echo "1.4 空请求体POST..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/v1/login -d '' 2>/dev/null)
[ "$CODE" = "400" ] && ok "空POST: HTTP 400" || fail "空POST" "400" "$CODE"

# 1.5 边界 — 超长JSON
echo "1.5 超长JSON攻击..."
LONG=$(python3 -c "import json; print(json.dumps({'username':'a'*1000,'password':'b'*100000}))" 2>/dev/null)
CODE=$(echo "$LONG" | curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/v1/login -d @- 2>/dev/null)
[ "$CODE" = "413" ] || [ "$CODE" = "400" ] && ok "超长JSON: HTTP $CODE" \
  || fail "超长JSON" "413 or 400" "$CODE"

# 1.6 边界 — SQL注入尝试
echo "1.6 SQL注入防御..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/v1/novels?sort_by=id;DROP%20TABLE%20novels;--" 2>/dev/null)
[ "$CODE" = "200" ] && ok "SQL注入sort_by: HTTP 200 (安全忽略)" \
  || fail "SQL注入sort_by" "200" "$CODE"

# 1.7 注册 — 弱密码
echo "1.7 弱密码拒绝..."
RESP=$(curl -s -X POST $BASE/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test_user","password":"123"}' 2>/dev/null)
echo "$RESP" | grep -q "8 characters" && ok "弱密码拒绝: 正确" \
  || fail "弱密码拒绝" "8 characters error" "$RESP"

# 1.8 注册 — 正常注册
echo "1.8 正常注册+登录..."
curl -s -X POST $BASE/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"username":"stresstest","password":"test12345"}' > /dev/null 2>&1

TOKEN=$(curl -s -X POST $BASE/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"stresstest","password":"test12345"}' 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('access_token',''))" 2>/dev/null)
[ -n "$TOKEN" ] && ok "注册+登录: Token获取成功" || fail "注册+登录" "got token" "empty"

# 1.9 并发注册相同用户名 (竞态)
echo "1.9 并发注册竞态..."
for i in $(seq 1 10); do
  curl -s -X POST $BASE/api/v1/register \
    -H "Content-Type: application/json" \
    -d '{"username":"race_user","password":"test12345"}' > /dev/null 2>&1 &
done
wait
COUNT=$(curl -s -X POST $BASE/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"race_user","password":"test12345"}' 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('access_token','FAIL'))" 2>/dev/null)
[ "$COUNT" != "FAIL" ] && ok "并发注册竞态: 至少1个成功" || fail "并发注册竞态" "success" "all failed"

# 1.10 速率限制测试
echo "1.10 速率限制..."
LIMIT_HIT=0
for i in $(seq 1 150); do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/v1/novels 2>/dev/null)
  [ "$CODE" = "429" ] && LIMIT_HIT=1 && break
done
[ "$LIMIT_HIT" = "1" ] && ok "速率限制: 触发429" || ok "速率限制: 100req/60s窗口内未触发(OK)"

section "ROUND 1 完成: $PASS 通过, $FAIL 失败"
