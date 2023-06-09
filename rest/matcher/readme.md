# 路由

本包仅用于 http 路由匹配，使用场景为路径参数解析。例如： /apis/{group}/{version}.

## 动机

原使用的 gin 框架，其路由匹配使用的前缀树实现，匹配速度较快。但是在使用过程中有许多的限制，
例如不能实现确定路径与通配路径同时存在（/api/core/v1 与 /{group}/{version}同时存在），相同前缀的路由无法被注册（/app1, /app2）.
无法在路径中注册带":"的路由（google api 中的自定义方法），以及其他复杂的匹配问题。

## 语法

- 使用 '{' 与'}'定义变量匹配，其间的字符作为变量名称。可以使用 "{}"定义无名称变量。可以使用 `:` 作为变量名称的结束符，后续的字符作为变量的正则表达式。
- 使用 '\*' 作为最后一个字符表示向后匹配。/{name}\*,将使 name 向后匹配。
- 其他字符作为常规字符进行匹配。

## 设计

- 使用前缀树实现，匹配速度较快。
- 在复杂匹配时退化为规则匹配。

举例：

在使用 `/api/{repository:(?:[a-zA-Z0-9]+(?:[.\_-][a-zA-Z0-9]+)_/?)+}_/manifests/{reference}` 作为路由时，其等效于
`/api/{repository}*/manifests/{reference}`.
`/api/{repository}*/digest/{digest}`.

匹配至带有 `*` 的节点时，将后续表达式提前计算，直至成功或者失败。

## 路由匹配

### 固定匹配

- /api/v1/

  - ✅/api/v1/
  - ❌/api/v1
  - ❌/api/v11
  - ❌/api/v789

- /api/v1\*

  - ✅/api/v1/
  - ✅/api/v1
  - ✅/api/v11
  - ✅/api/v11/hello
  - ✅/api/v1/hello
  - ❌/api/v789

- /api/\*1 (非后缀的\*作为普通字符对待)

  - ✅/api/\*1
  - ❌/api/v1
  - ❌/api/v/1

### 变量匹配

- /api/{version}

  - ✅/api/\*1;version=\*1
  - ✅/api/v1;version=v1
  - ❌/api/v/1
  - ❌/api/v1/

- /api/v{version}

  - ✅/api/v1;version=1
  - ✅/api/vhello;version=hello
  - ✅/api/v\*1;version=\*1
  - ❌/api/v1/
  - ❌/api/v/1

- /api/v{version}\*

  - ✅/api/v1;version=1
  - ✅/api/vhello/world;version=hello/world
  - ✅/api/v/1;version=/1
  - ❌/apis/v1

- /api/v{version}/{group}

  - ✅/api/v1/;version=1,group=
  - ✅/api/vhello/world;version=hello,group=world
  - ❌/api/v1;version=1
  - ✅/api/v/1;version=,group=1

### 增强变量匹配

- /person/{firstname:[a-z]{3}}-{lastname}

  - ✅/person/tom-cat;firstname=tom,lastname=cat
  - ❌/person/jack-ma/
  - ❌/person/jackma

- /{repository}\*/manifest/{reference}
  - ✅/library/nginx/manifest/1.0;repository=library/nginx,reference=1.0
  - ✅/library/a/b/c/manifest/1.0;repository=library/a/b/c,reference=1.0

### 向后匹配

- /prefix/{path}\*

  - ✅/prefix/abc;path=abc
  - ✅/prefix/abc/def;path=abc/def
  - ❌/prefixabc;

- /prefix\*

  - ✅/prefix/abc
  - ✅/prefixabc/def
  - ❌/pref/jackma

### 混合使用

若同时定义如下路由：

- /api/v{version}/{group}
- /api/v1/{group}
- /api/v1/core

则若同时满足上述条件下，路径**参数少**的匹配被选中。

举例：

- 请求/api/v1/core,则匹配中 /api/v1/core;
- 请求/api/v1/hello,则匹配中 /api/v1/{group};
- 请求/api/v2/hello,则匹配中 /api/v{version}/{group};
