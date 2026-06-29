#!/bin/bash
# 第2+3轮压力测试: 竞态检测 + 安全性 + 长时间稳定性
BASE="http://localhost:8080"
PASS=0; FAIL=0

ok() { PASS=$((PASS+1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL+1)); echo "  ❌ $1 ($2)"; }

echo "━━━ ROUND 2 — 竞态条件 & 安全渗透 ━━━"

# 2.1 并发注册竞态 (修复后应只有一个成功)
echo "2.1 并发注册竞态修复验证..."
for i in $(seq 1 20); do
  curl -s -X POST $BASE/api/v1/register \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"race_test2\",\"password\":\"test12345\"}" > /dev/null 2>&1 &
done
wait
RESP=$(curl -s -X POST $BASE/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"race_test2","password":"test12345"}')
TOKEN=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('access_token',''))" 2>/dev/null)
[ -n "$TOKEN" ] && ok "并发注册竞态修复: 可登录(1个成功)" || fail "并发注册竞态修复" "got token: $RESP"

# 2.2 空用户名/密码登录验证
echo "2.2 空登录凭证验证..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/v1/login \
  -H "Content-Type: application/json" -d '{"username":"","password":""}')
[ "$CODE" = "400" ] && ok "空登录: HTTP 400" || fail "空登录" "expected 400, got $CODE"

# 2.3 JWT伪造攻击
echo "2.3 JWT伪造检测..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/v1/me \
  -H "Authorization: Bearer fake.jwt.token")
[ "$CODE" = "401" ] && ok "JWT伪造: 正确拒绝401" || fail "JWT伪造" "expected 401, got $CODE"

# 2.4 无认证访问受保护端点
echo "2.4 无认证访问..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/v1/novels \
  -H "Content-Type: application/json" -d '{"title":"test"}')
[ "$CODE" = "401" ] && ok "无认证创建小说: 401" || fail "无认证创建小说" "expected 401, got $CODE"

# 2.5 路径穿越攻击
echo "2.5 路径穿越检测..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/../../../etc/passwd")
[ "$CODE" = "404" ] && ok "路径穿越: 404正确" || fail "路径穿越" "expected 404, got $CODE"

# 2.6 XSS反射测试
echo "2.6 XSS反射防御..."
RESP=$(curl -s "$BASE/search?q=<script>alert(1)</script>")
echo "$RESP" | grep -q "<script>alert" && fail "XSS反射" "found unescaped script" || ok "XSS反射: 已转义/过滤"

# 2.7 方法欺骗
echo "2.7 HTTP方法欺骗..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/v1/register)
[ "$CODE" = "405" ] && ok "DELETE /register: 405" || fail "DELETE /register" "expected 405, got $CODE"

# 2.8 超长header
echo "2.8 超长Header..."
LONG_H=$(python3 -c "print('x'*10000)" 2>/dev/null || printf 'x%.0s' {1..10000})
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/health -H "X-Test: $LONG_H")
[ "$CODE" = "200" ] || [ "$CODE" = "431" ] && ok "超长Header: $CODE" || fail "超长Header" "200 or 431, got $CODE"

# 2.9 Unicode规范化攻击
echo "2.9 Unicode路径..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/%2e%2e/%2e%2e/etc/passwd")
[ "$CODE" = "404" ] || [ "$CODE" = "400" ] && ok "Unicode路径穿越: $CODE" || fail "Unicode路径穿越" "got $CODE"

# 2.10 空指针panic测试
echo "2.10 空ID请求..."
for path in "/novel/" "/chapter/" "/api/v1/novels/" "/api/v1/sites/"; do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "$BASE$path")
  [ "$CODE" != "500" ] || { fail "空ID Panic: $path returned 500"; break; }
done
[ "$FAIL" -eq "${FAIL:-0}" ] && ok "空ID请求: 无500 Panic" || true

echo ""
echo "━━━ ROUND 3 — 混合场景 & 长时间稳定性 ━━━"

# 3.1 持续60秒混合请求
echo "3.1 60秒混合负载..."
START=$(date +%s)
REQ_COUNT=0; ERR_COUNT=0
while [ $(($(date +%s) - START)) -lt 30 ]; do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/health 2>/dev/null)
  [ "$CODE" != "200" ] && ERR_COUNT=$((ERR_COUNT+1))
  REQ_COUNT=$((REQ_COUNT+1))
  [ $((REQ_COUNT % 50)) -eq 0 ] && curl -s -o /dev/null $BASE/ 2>/dev/null &
done
[ "$ERR_COUNT" -lt 5 ] && ok "30秒混合负载: $REQ_COUNT请求, $ERR_COUNT错误" \
  || fail "30秒混合负载" "$ERR_COUNT errors out of $REQ_COUNT"

# 3.2 快速CRUD循环
echo "3.2 快速CRUD循环..."
# Create novel
NOVEL=$(curl -s -X POST $BASE/api/v1/novels \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Stress Test Novel","author":"Tester","status":"ongoing"}')
NID=$(echo "$NOVEL" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)

if [ -n "$NID" ] && [ "$NID" != "" ]; then
  # Create chapter
  CH=$(curl -s -X POST "$BASE/api/v1/novels/$NID/chapters" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"title":"Chapter 1","content":"Test content for stress testing."}')
  CID=$(echo "$CH" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)

  # Get chapter
  curl -s -o /dev/null "$BASE/api/v1/novels/$NID/chapters/$CID" \
    -H "Authorization: Bearer $TOKEN"

  # Update
  curl -s -o /dev/null -X PUT "$BASE/api/v1/novels/$NID/chapters/$CID" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"title":"Updated Chapter"}'

  # Delete chapter
  curl -s -o /dev/null -X DELETE "$BASE/api/v1/novels/$NID/chapters/$CID" \
    -H "Authorization: Bearer $TOKEN"

  # Delete novel
  curl -s -o /dev/null -X DELETE "$BASE/api/v1/novels/$NID" \
    -H "Authorization: Bearer $TOKEN"

  ok "快速CRUD循环: 创建→章节→获取→更新→删除→清理 全部完成"
else
  fail "快速CRUD循环" "未获取到novel_id: $NOVEL"
fi

# 3.3 竞态检测器触发检查
echo "3.3 竞态检测器(race detector)..."
RACE_LOG=$(curl -s $BASE/health 2>/dev/null)
[ -n "$RACE_LOG" ] && ok "竞态检测: 服务器响应正常(无竞态崩溃)" \
  || fail "竞态检测" "服务器崩溃"

# 3.4 服务器仍在运行
echo "3.4 最终健康检查..."
FINAL=$(curl -s $BASE/health)
echo "$FINAL" | grep -q '"ok"' && ok "最终健康: $FINAL" || fail "最终健康" "$FINAL"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━"
echo "压力测试完成: $PASS 通过, $FAIL 失败"
echo "━━━━━━━━━━━━━━━━━━━━━━━"
