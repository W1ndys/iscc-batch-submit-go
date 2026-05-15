# ISCC 平台 API 文档

> 基于 [Reqable](https://reqable.com) 抓包与 `iscc-batch-submit-go` 代码实现整理。
>
> 基准地址：`https://iscc.isclab.org.cn`（可通过 `--base-url` 覆盖）

---

## 目录

1. [认证与会话](#1-认证与会话)
2. [练武题（Regular Challenges）](#2-练武题regular-challenges)
3. [擂台题（Arena Challenges）](#3-擂台题arena-challenges)
4. [成绩查询 APIs](#4-成绩查询-apis)
5. [Flag 提交流程与返回码](#5-flag-提交流程与返回码)
6. [附录：通用请求头与响应格式](#6-附录通用请求头与响应格式)

---

## 1. 认证与会话

### 1.1 登录

```
POST /login
Content-Type: application/x-www-form-urlencoded
```

**请求头**

| 头字段 | 值 |
|--------|-----|
| User-Agent | Mozilla/5.0 ... |
| Accept | text/html,application/xhtml+xml,... |
| Content-Type | application/x-www-form-urlencoded |
| Origin | `https://iscc.isclab.org.cn` |
| Referer | `https://iscc.isclab.org.cn/login` |

**请求体**（表单编码）

```
name=<用户名>&password=<密码>
```

**响应**

- `200 OK` — 登录成功。服务端通过 `Set-Cookie` 下发会话 Cookie（`session`）
- `>= 400` — 登录失败

> 登录后的 `session` Cookie 需要附加到后续所有请求中。

### 1.2 Cookie 缓存

平台使用 `session` Cookie 维持登录态，Cookie 域为 `iscc.isclab.org.cn`。工具可将 Cookie 序列化为 JSON 缓存：

```json
{
  "base_url": "https://iscc.isclab.org.cn",
  "saved_at": 1778575716,
  "cookies": [
    {
      "name": "session",
      "value": "affda5f7-1e1c-4232-850d-9bca0eda9f7b",
      "domain": "",
      "path": "/"
    }
  ]
}
```

---

## 2. 练武题（Regular Challenges）

### 2.1 获取练武题页面

```
GET /challenges
Referer: https://iscc.isclab.org.cn/
```

返回 HTML 页面，从中可解析两类信息：

#### 2.1a 解析 CSRF Nonce

页面中包含 `<input name="nonce" value="...">`，用于后续 flag 提交的反跨站请求伪造保护。

```html
<input type="hidden" id="nonce" name="nonce" value="56147480...">
```

解析方式：遍历 DOM 中所有 `input` 元素，查找 `name` 为 `nonce` 或 `csrf_nonce` 的节点，取其 `value` 属性。也支持正则回退匹配。

#### 2.1b 解析队伍路径

通过查找 `<a href="/team/{hexid}">` 提取队伍路径，用于后续获取已解题目列表。

```html
<a href="/team/b5fb4ac70c4d684db5f2ffced81b0b42">W1ndys</a>
```

### 2.2 获取练武题列表

```
GET /chals
X-Requested-With: XMLHttpRequest
Accept: application/json
Referer: https://iscc.isclab.org.cn/challenges
```

**响应**（JSON）

```json
{
  "game": [
    { "category": "WEB", "id": 31, "value": 400 },
    { "category": "MISC", "id": 52, "value": 400 }
  ]
}
```

> 该接口返回题目 ID、分类和分值。`value` 是分值，不是题目名称。题目名称需要调用 2.3 逐个获取。

### 2.3 获取练武题详情

```
GET /chals/{challengeID}
X-Requested-With: XMLHttpRequest
Accept: application/json
Referer: https://iscc.isclab.org.cn/challenges
```

**响应**（JSON）

```json
{
  "author": "W1ndys",
  "category": "MISC",
  "description": "题目描述...",
  "files": [],
  "id": 31,
  "name": "题目名称",
  "solves": 0,
  "value": 400
}
```

关键字段：
- `name` — 题目名称
- `value` — 题目分值
- `solves` — 当前已解人数（用于优先级排序）

### 2.4 获取已解题目（练武题）

```
GET /solves/{teamID}
Accept: application/json
Referer: https://iscc.isclab.org.cn/team/{teamID}
```

`teamID` 是从 `/challenges` 页面 `<a href="/team/{teamID}">` 中提取的十六进制字符串。

**响应示例**

```json
{
  "solves": [
    {
      "chalid": 11,
      "chal": "Web 11",
      "category": "WEB",
      "team": 7167,
      "value": 120,
      "time": 1778298714
    }
  ]
}
```

### 2.5 提交练武题 Flag

```
POST /chal/{challengeID}
Content-Type: application/x-www-form-urlencoded; charset=UTF-8
X-Requested-With: XMLHttpRequest
Accept: */*
Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
Origin: https://iscc.isclab.org.cn
Referer: https://iscc.isclab.org.cn/challenges
```

**请求体**

```
key={flag字符串}&nonce={从/challenges页面获取的nonce}
```

> nonce 需要通过前置 GET /challenges 获取，每次提交前都应当重新获取。

**响应**

纯数字状态码。详见[第 5 节](#5-flag-提交流程与返回码)。

---

## 3. 擂台题（Arena Challenges）

> 擂台题与练武题共享同一套认证会话（session Cookie），但 nonce、题目列表、提交接口均独立。

### 3.1 获取擂台题页面（用于获取 nonce）

```
GET /arena
Referer: https://iscc.isclab.org.cn/
```

返回 HTML 页面，从中解析 nonce（与练武题相同方式）：

```html
<input type="hidden" id="nonce" name="nonce" value="56147480b0ee92...">
```

### 3.2 获取擂台题列表

```
GET /arenas
X-Requested-With: XMLHttpRequest
Referer: https://iscc.isclab.org.cn/arena
```

**响应**（JSON）

```json
{
  "game": [
    { "category": "MISC",    "id": 1,  "value": 150 },
    { "category": "MOBILE",  "id": 2,  "value": 150 },
    { "category": "PWN",     "id": 3,  "value": 150 },
    { "category": "REVERSE", "id": 4,  "value": 150 },
    { "category": "WEB",     "id": 5,  "value": 150 },
    { "category": "PWN",     "id": 6,  "value": 150 },
    { "category": "WEB",     "id": 7,  "value": 150 },
    { "category": "MISC",    "id": 8,  "value": 150 },
    { "category": "REVERSE", "id": 9,  "value": 150 },
    { "category": "MOBILE",  "id": 10, "value": 150 },
    { "category": "REVERSE", "id": 11, "value": 150 },
    { "category": "MISC",    "id": 12, "value": 150 },
    { "category": "PWN",     "id": 13, "value": 150 },
    { "category": "MISC",    "id": 14, "value": 150 }
  ]
}
```

> 该接口仅返回题目 ID、分类和分值，`value` 是分值，不是题目名称。
> 名称和已解人数需要调用 3.4 逐个获取。

### 3.3 获取已解擂台题

```
GET /arenasolves
X-Requested-With: XMLHttpRequest
Accept: application/json
Referer: https://iscc.isclab.org.cn/arena
```

**响应**（JSON）

```json
{
  "solves": [
    {
      "category": "WEB",
      "chal": "数字古墓",
      "chalid": 5,
      "id": 1703,
      "team": 7167,
      "time": 1778298714,
      "value": 120
    },
    {
      "category": "MOBILE",
      "chal": "深海金库",
      "chalid": 2,
      "id": 1717,
      "team": 7167,
      "time": 1778298721,
      "value": 120
    }
  ]
}
```

### 3.4 获取擂台题详情

```
GET /arenas/{challengeID}
X-Requested-With: XMLHttpRequest
Accept: application/json
Referer: https://iscc.isclab.org.cn/arena
```

**响应**（JSON）

```json
{
  "author": "YanamiAnna",
  "category": "PWN",
  "description": "本题是一道双层虚拟机挑战，由EVM（自定义RISC-V风格模拟器）和BlindVM（堆虚拟机）两部分组成。\n题目地址:39.96.193.120:8888",
  "files": [
    "static/uploads/d55f31c4312bede1f0b3e2533044eda7/ld-linux-x86-64.so.zip"
  ],
  "id": 13,
  "name": "EVM",
  "solves": 0,
  "value": 150
}
```

关键字段：
- `name` — 题目名称
- `value` — 题目分值
- `solves` — 当前已解人数（用于优先级排序）

### 3.5 提交擂台题 Flag

```
POST /are/{challengeID}
Content-Type: application/x-www-form-urlencoded; charset=UTF-8
X-Requested-With: XMLHttpRequest
Accept: */*
Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
Origin: https://iscc.isclab.org.cn
Referer: https://iscc.isclab.org.cn/arena
```

**请求体**

```
key={flag字符串}&nonce={从/arena页面获取的nonce}
```

> 与练武题不同：请求路径为 `/are/{id}`（非 `/arena/{id}`），nonce 来源为 `/arena` 页面（非 `/challenges`）。

**响应**

与练武题一致，纯数字状态码。详见[第 5 节](#5-flag-提交流程与返回码)。

---

## 4. 成绩查询 APIs

### 4.1 练武题个人成绩页面

```
GET /team/{teamID}
Referer: https://iscc.isclab.org.cn/team/{teamID}
```

`teamID` 为十六进制字符串（如 `b5fb4ac70c4d684db5f2ffced81b0b42`），可从任意页面导航栏 `<a href="/team/{teamID}">` 获取。

返回 HTML 页面，包含：
- 总积分与排名（`总积分为:1440,排在602位。`）
- 已解题目表格（题目名称、分类、得分、通过时间）
- 题目分类饼图、得分趋势图

### 4.2 练武题已解详情

```
GET /solves/{teamID}
X-Requested-With: XMLHttpRequest
Accept: */*
Referer: https://iscc.isclab.org.cn/team/{teamID}
```

> 与 2.2 是同一接口，但 Referer 不同。返回结构一致。

```json
{
  "solves": [
    {
      "chalid": 11,
      "chal": "Web 11",
      "category": "WEB",
      "team": 7167,
      "value": 120,
      "time": 1778298714
    }
  ]
}
```

### 4.3 练武题提交统计

```
GET /fails/{teamID}
X-Requested-With: XMLHttpRequest
Accept: */*
Referer: https://iscc.isclab.org.cn/team/{teamID}
```

```json
{
  "fails": "10",
  "solves": "12"
}
```

- `fails` — 提交失败次数
- `solves` — 成功解题数

### 4.4 擂台题个人成绩页面

```
GET /teamarena/{teamID}
Referer: https://iscc.isclab.org.cn/team/{teamID}
```

返回 HTML 页面，内容与 4.1 类似，但展示的是擂台赛成绩：
- 总积分与排名
- 已解擂台题表格（擂台名称、分类、得分、通过时间）
- 题目链接格式：`<a href="/arena#题目名">`

### 4.5 擂台题个人已解详情

```
GET /arenasolves/{teamID}
X-Requested-With: XMLHttpRequest
Accept: */*
Referer: https://iscc.isclab.org.cn/teamarena/{teamID}
```

> 与 3.3 的全局 `/arenasolves` 接口返回结构一致，但加上了 `{teamID}` 路径参数限定到指定队伍。

```json
{
  "solves": [
    {
      "category": "WEB",
      "chal": "数字古墓",
      "chalid": 5,
      "id": 1703,
      "team": 7167,
      "time": 1778298714,
      "value": 120
    }
  ]
}
```

### 4.6 擂台题提交统计

```
GET /arenafails/{teamID}
X-Requested-With: XMLHttpRequest
Accept: */*
Referer: https://iscc.isclab.org.cn/teamarena/{teamID}
```

```json
{
  "fails": "10",
  "solves": "12"
}
```

---

## 5. Flag 提交流程与返回码

### 响应格式

Flag 提交接口的响应为**纯数字**（字符串形式），无 JSON 包装：

```
1
```

### 返回码对照表

| 返回码 | 含义 | 处理建议 |
|--------|------|---------|
| `1` | 正确（Accepted） | 停止尝试该题目 |
| `0` | 错误（Wrong） | 继续尝试下一个 flag |
| `2` | 已解决过（Already Solved） | 停止尝试该题目 |
| `3` | 速度太快（Rate Limited） | 等待 `throttle-delay` 后重试 |
| `4` | 题目未开放或无权限 | 跳过该题目 |
| `5` | 服务器错误或未知错误 | 可重试 |
| `-1` | Nonce 错误或登录状态失效 | 重新获取 nonce 或重新登录 |

### 完整提交流程

```
1. 登录（POST /login）
       ↓
2. 获取 nonce（GET /challenges 或 GET /arena）
       ↓
3. 获取题目列表（GET /chals 或 GET /arenas）
       ↓
4. 获取题目详情（GET /chals/{id} 或 GET /arenas/{id}，按需读取 name）
       ↓
5. 获取已解题目列表（GET /solves/{teamID} 或 GET /arenasolves）
       ↓
6. 并发提交未解题目（POST /chal/{id} 或 POST /are/{id}）
       ↓
7. 解析响应码，标记已解题目
       ↓
8. 重复 2~7 直到全部解出或达到最大重试轮数
```

---

## 6. 附录：通用请求头与响应格式

### 通用请求头

以下请求头在大部分 API 请求中固定使用：

| 头字段 | 值 |
|--------|-----|
| User-Agent | `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 ...` |
| Connection | `close`（GET 请求） |
| Accept-Language | `zh-CN,zh;q=0.9,en;q=0.8` |
| Sec-Fetch-Dest | `empty` 或 `document` |
| Sec-Fetch-Mode | `cors` 或 `navigate` |
| Sec-Fetch-Site | `same-origin` |

### 服务端信息

- **服务器**: `nginx/1.10.3 (Ubuntu)`
- **IP**: `39.96.201.215`
- **编码**: gzip（`Content-Encoding: gzip`，需要客户端自动解压）

### 页面导航路径

```
/              → 首页
/login         → 登录页
/logout        → 登出
/rule          → 竞赛规则
/choice        → 理论题
/challenges    → 练武题（含 nonce）
/arena         → 擂台题（含 nonce）
/measure       → 实战题
/bigdata       → 数据安全赛
/scoreboard    → 练武积分板
/arenascoreboard → 擂台积分板
/notice        → 公告栏
/study         → 学习资料
/thanks        → 致谢
/team/{id}     → 队伍页面（练武成绩）
/teamarena/{id} → 队伍页面（擂台成绩）
```

---

## 修订历史

| 日期 | 变更 |
|------|------|
| 2026-05-12 | 初稿，基于 Reqable 抓包与 `iscc-batch-submit-go` v1 代码整理 |
| 2026-05-12 | 新增第 4 节「成绩查询 APIs」，收录 `/team`、`/fails`、`/teamarena`、`/arenasolves/{id}`、`/arenafails` |
| 2026-05-15 | 补充 `/chals` 与 `/chals/{id}`，明确列表接口 `value` 为分值，题目名称需通过详情接口获取 |
