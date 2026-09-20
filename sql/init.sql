-- ============================================================
-- 家庭物品整理收纳系统 · 初始化脚本（MySQL 8.4）
-- 鉴权体系依据：auth-system-design/登录鉴权系统设计稿.html v1.0
-- 业务模型：家庭（共享物品库）+ 三级角色
--   admin         超级管理员：管理所有家庭、所有用户（管理后台）
--   family_admin  家庭管理员：管理本家庭成员 + 本家庭信息全权，可移交
--   member        普通成员：查看本家庭全部信息，仅能改/删自己提交的
-- 密码哈希：Argon2id (m=19456, t=2, p=1) PHC 格式；占位符由 server/cmd/hashpw 生成后替换
-- ============================================================

CREATE DATABASE IF NOT EXISTS home_items
  DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
USE home_items;

-- ---------- 家庭表 ----------
CREATE TABLE IF NOT EXISTS sys_family (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name       VARCHAR(64)  NOT NULL COMMENT '家庭名',
  remark     VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted    TINYINT      NOT NULL DEFAULT 0,
  UNIQUE KEY uk_name (name)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '家庭表';

-- ---------- 用户表 ----------
CREATE TABLE IF NOT EXISTS sys_user (
  id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  username      VARCHAR(64)  NOT NULL,
  password_hash VARCHAR(255) NOT NULL COMMENT 'Argon2id 哈希（PHC 格式，含算法参数与盐）',
  nickname      VARCHAR(64)  NOT NULL DEFAULT '',
  email         VARCHAR(255) NULL,
  family_id     BIGINT UNSIGNED NULL COMMENT '所属家庭；admin 无家庭',
  status        TINYINT      NOT NULL DEFAULT 1 COMMENT '0禁用 1正常',
  last_login_at DATETIME     NULL,
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted       TINYINT      NOT NULL DEFAULT 0 COMMENT '软删除标记',
  UNIQUE KEY uk_username (username),
  KEY idx_family (family_id),
  CONSTRAINT fk_user_family FOREIGN KEY (family_id) REFERENCES sys_family (id),
  CONSTRAINT chk_user_status CHECK (status IN (0, 1))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '用户表';

-- ---------- 角色表 ----------
CREATE TABLE IF NOT EXISTS sys_role (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  code       VARCHAR(64)  NOT NULL COMMENT '角色编码：admin / family_admin / member',
  name       VARCHAR(64)  NOT NULL COMMENT '显示名',
  remark     VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_code (code)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '角色表';

-- ---------- 权限表（权限码是贯穿前后端的唯一契约字符串） ----------
-- 行级范围（全部 / 本家庭 / 本人提交）不靠权限码表达，由 handler 按角色 + family_id
-- 下沉到 SQL 过滤 —— 对应设计稿 §4.2 "data_scope 下沉到 SQL" 的思想。
CREATE TABLE IF NOT EXISTS sys_permission (
  id        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  code      VARCHAR(128) NOT NULL COMMENT '权限码：sys:user:list / biz:item:create',
  name      VARCHAR(64)  NOT NULL,
  type      VARCHAR(16)  NOT NULL COMMENT 'MENU / API / BUTTON',
  parent_id BIGINT UNSIGNED NULL COMMENT '菜单树父节点',
  path      VARCHAR(255) NULL COMMENT '路由路径（MENU）或接口路径（API）',
  component VARCHAR(255) NULL COMMENT '前端组件路径（MENU 用）',
  icon      VARCHAR(64)  NULL,
  sort      INT          NOT NULL DEFAULT 0,
  status    TINYINT      NOT NULL DEFAULT 1,
  UNIQUE KEY uk_code (code),
  KEY idx_parent (parent_id),
  CONSTRAINT chk_perm_type CHECK (type IN ('MENU', 'API', 'BUTTON'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '权限表';

-- ---------- 用户-角色 ----------
CREATE TABLE IF NOT EXISTS sys_user_role (
  user_id BIGINT UNSIGNED NOT NULL,
  role_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (user_id, role_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '用户-角色';

-- ---------- 角色-权限 ----------
CREATE TABLE IF NOT EXISTS sys_role_permission (
  role_id       BIGINT UNSIGNED NOT NULL,
  permission_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (role_id, permission_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '角色-权限';

-- ---------- 审计日志 ----------
CREATE TABLE IF NOT EXISTS sys_audit_log (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id    BIGINT UNSIGNED NULL COMMENT '操作人，登录失败时可能无对应用户',
  username   VARCHAR(64)  NOT NULL DEFAULT '',
  action     VARCHAR(64)  NOT NULL COMMENT 'LOGIN_OK / LOGIN_FAIL / USER_DELETE / TRANSFER_ADMIN / REUSE_DETECTED ...',
  target     VARCHAR(128) NOT NULL DEFAULT '',
  detail     VARCHAR(512) NOT NULL DEFAULT '',
  ip         VARCHAR(64)  NOT NULL DEFAULT '',
  ua         VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user (user_id),
  KEY idx_action_time (action, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '审计日志';

-- ---------- 分类表（家庭级，物品录入时可选） ----------
CREATE TABLE IF NOT EXISTS biz_category (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  family_id  BIGINT UNSIGNED NOT NULL COMMENT '归属家庭',
  name       VARCHAR(64)    NOT NULL,
  sort       INT            NOT NULL DEFAULT 0,
  created_at DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_family_name (family_id, name),
  CONSTRAINT fk_cat_family FOREIGN KEY (family_id) REFERENCES sys_family (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '物品分类表';

-- ---------- 物品表 ----------
-- creator_id = 原始提交人（展示"谁提交的"，历史不变）
-- owner_id   = 当前责任人（可改可删这条信息的人；删除用户时转移给接收人）
-- 家庭内所有成员可见（查询按 family_id 过滤），member 只能管理 owner 是自己的
CREATE TABLE IF NOT EXISTS biz_item (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  family_id   BIGINT UNSIGNED NOT NULL COMMENT '归属家庭（可见范围）',
  name        VARCHAR(128)  NOT NULL COMMENT '物品名称',
  quantity    INT           NOT NULL DEFAULT 1 COMMENT '数量',
  category_id BIGINT UNSIGNED NULL COMMENT '分类，可空 = 未分类',
  image       VARCHAR(255)  NULL COMMENT '图片相对路径 /uploads/xxx.jpg，可空',
  remark      VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '备注',
  creator_id  BIGINT UNSIGNED NOT NULL COMMENT '原始提交人',
  owner_id    BIGINT UNSIGNED NOT NULL COMMENT '当前责任人（增删改权随它走）',
  created_at  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted     TINYINT       NOT NULL DEFAULT 0 COMMENT '软删除',
  KEY idx_family_name (family_id, name),
  KEY idx_family_cat (family_id, category_id),
  KEY idx_owner (owner_id),
  CONSTRAINT fk_item_family  FOREIGN KEY (family_id)  REFERENCES sys_family (id),
  CONSTRAINT fk_item_creator FOREIGN KEY (creator_id) REFERENCES sys_user (id),
  CONSTRAINT fk_item_owner   FOREIGN KEY (owner_id)   REFERENCES sys_user (id),
  CONSTRAINT fk_item_cat     FOREIGN KEY (category_id) REFERENCES biz_category (id),
  CONSTRAINT chk_qty CHECK (quantity >= 0)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '物品表';

-- ---------- 物品修改历史（详情页"修改历史"数据源） ----------
-- 每次变更记一条：action = CREATE / UPDATE / DELETE / TRANSFER
-- before / after 存变更字段的 JSON 快照，审计与回溯两用
CREATE TABLE IF NOT EXISTS biz_item_history (
  id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  item_id       BIGINT UNSIGNED NOT NULL,
  operator_id   BIGINT UNSIGNED NOT NULL COMMENT '操作人',
  operator_name VARCHAR(64)    NOT NULL,
  action        VARCHAR(16)    NOT NULL COMMENT 'CREATE / UPDATE / DELETE / TRANSFER',
  before_json   JSON           NULL COMMENT '变更前快照（关键业务字段）',
  after_json    JSON           NULL COMMENT '变更后快照',
  created_at    DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_item (item_id, id),
  CONSTRAINT fk_hist_item FOREIGN KEY (item_id) REFERENCES biz_item (id),
  CONSTRAINT chk_hist_action CHECK (action IN ('CREATE', 'UPDATE', 'DELETE', 'TRANSFER'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '物品修改历史';

-- ============================================================
-- 种子数据
-- ============================================================

INSERT INTO sys_family (id, name, remark) VALUES
  (1, '张家', '张三一家'),
  (2, '李家', '李四一家');

-- ---- 权限码（前后端唯一契约） ----
-- 菜单
INSERT INTO sys_permission (id, code, name, type, parent_id, path, component, icon, sort) VALUES
  (100, 'dashboard',          '首页',     'MENU', NULL, '/dashboard',       'dashboard/index',    'Odometer', 1),
  (110, 'system',             '系统管理', 'MENU', NULL, '/system',          NULL,                 'Setting',  2),
  (111, 'sys:family:page',    '家庭管理', 'MENU', 110,  '/system/families', 'system/family/index','House',    1),
  (112, 'sys:user:page',      '用户管理', 'MENU', 110,  '/system/users',    'system/users/index', 'User',     2),
  (113, 'sys:audit:page',     '审计日志', 'MENU', 110,  '/system/audit',    'system/audit/index', 'Document', 3),
  (120, 'biz:item:page',      '家庭物品', 'MENU', NULL, '/items',           'item/index',         'Box',      3),
  (121, 'biz:category:page',  '分类管理', 'MENU', NULL, '/categories',      'category/index',     'Files',    4);

-- 管理端接口
INSERT INTO sys_permission (id, code, name, type, parent_id, path, sort) VALUES
  (1111, 'sys:family:list',     '家庭列表',   'API', 111, '/api/v1/admin/families', 1),
  (1112, 'sys:family:create',   '创建家庭',   'API', 111, '/api/v1/admin/families', 2),
  (1113, 'sys:family:update',   '更新家庭',   'API', 111, '/api/v1/admin/families/:id', 3),
  (1114, 'sys:family:delete',   '删除家庭',   'API', 111, '/api/v1/admin/families/:id', 4),
  (1115, 'sys:family:transfer', '移交管理员', 'API', 111, '/api/v1/admin/families/:id/transfer', 5),
  (1121, 'sys:user:list',       '用户列表',   'API', 112, '/api/v1/admin/users', 1),
  (1122, 'sys:user:create',     '创建用户',   'API', 112, '/api/v1/admin/users', 2),
  (1123, 'sys:user:update',     '更新用户',   'API', 112, '/api/v1/admin/users/:id', 3),
  (1124, 'sys:user:delete',     '删除用户',   'API', 112, '/api/v1/admin/users/:id', 4),
  (1131, 'sys:audit:list',      '审计查询',   'API', 113, '/api/v1/admin/audit-logs', 1);

-- 业务端接口
INSERT INTO sys_permission (id, code, name, type, parent_id, path, sort) VALUES
  (1211, 'biz:item:list',       '物品列表',   'API', 120, '/api/v1/items', 1),
  (1212, 'biz:item:create',     '录入物品',   'API', 120, '/api/v1/items', 2),
  (1213, 'biz:item:update',     '编辑物品',   'API', 120, '/api/v1/items/:id', 3),
  (1214, 'biz:item:delete',     '删除物品',   'API', 120, '/api/v1/items/:id', 4),
  (1221, 'biz:category:list',   '分类列表',   'API', 121, '/api/v1/categories', 1),
  (1222, 'biz:category:create', '新建分类',   'API', 121, '/api/v1/categories', 2),
  (1223, 'biz:category:update', '编辑分类',   'API', 121, '/api/v1/categories/:id', 3),
  (1224, 'biz:category:delete', '删除分类',   'API', 121, '/api/v1/categories/:id', 4);

-- 按钮（BUTTON，管理端的"受控按钮清单"；code 用 btn: 前缀避免与 API 码撞唯一键。
-- 前端按钮显隐直接复用对应 API 码 —— 能调接口才显示按钮，天然一致）
INSERT INTO sys_permission (id, code, name, type, parent_id, sort) VALUES
  (2111, 'btn:sys:user:delete',     '删除用户按钮',   'BUTTON', 112, 1),
  (2112, 'btn:sys:family:transfer', '移交管理员按钮', 'BUTTON', 111, 1),
  (2211, 'btn:biz:item:delete',     '删除物品按钮',   'BUTTON', 120, 1);

-- ---- 角色 ----
INSERT INTO sys_role (id, code, name, remark) VALUES
  (1, 'admin',        '超级管理员', '管理所有家庭、所有用户'),
  (2, 'family_admin', '家庭管理员', '管理本家庭成员，本家庭信息全权，可移交'),
  (3, 'member',       '普通成员',   '查看本家庭全部信息，仅本人提交的可改删');

-- admin：全部权限
INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, id FROM sys_permission WHERE status = 1;

-- family_admin：用户管理（范围在 handler 限定本家庭）+ 移交 + 物品/分类全量
INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 2, id FROM sys_permission
WHERE code IN ('dashboard', 'sys:user:page', 'sys:family:transfer',
               'biz:item:page', 'biz:category:page')
   OR code LIKE 'sys:user:%'
   OR code LIKE 'biz:%';

-- member：物品查看/录入 + 只读分类（改删自己的由 handler 校验 owner）
INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 3, id FROM sys_permission
WHERE code IN ('dashboard', 'biz:item:page', 'biz:category:page',
               'biz:item:list', 'biz:item:create', 'biz:item:update', 'biz:item:delete',
               'biz:category:list');

-- ---- 用户（密码哈希为 Argon2id 占位，由 server/cmd/hashpw 生成后替换） ----
INSERT INTO sys_user (id, username, password_hash, nickname, email, family_id, status) VALUES
  (1, 'admin',     '$argon2id$v=19$m=19456,t=2,p=1$Yg/k8lstM6YUnlRPxZf/Dg$4q93J7qlzm7l6JwDPrVjKye/SVVwW553ruyY9YgcdoY',  '系统管理员', 'admin@example.com',     NULL, 1),
  (2, 'zhangsan',  '$argon2id$v=19$m=19456,t=2,p=1$QRBnkxsoLN0oDyt4xtvdmA$fPORZCcFz0NBnN7GfSOP414iogBdDoBHqL/d6cGo7fs',  '张三',       'zhangsan@example.com',  1,    1),
  (3, 'zhangmei',  '$argon2id$v=19$m=19456,t=2,p=1$+xhYgGioIGFfPhLmDontNA$jV6UkPlHVwP5CHy9EDSnwf73PbvI63nJ0P4iyXNmsHc',    '张小妹',     'zhangmei@example.com',  1,    1),
  (4, 'lisi',      '$argon2id$v=19$m=19456,t=2,p=1$QRBnkxsoLN0oDyt4xtvdmA$fPORZCcFz0NBnN7GfSOP414iogBdDoBHqL/d6cGo7fs',     '李四',       'lisi@example.com',      2,    1),
  (5, 'liwang',    '$argon2id$v=19$m=19456,t=2,p=1$+xhYgGioIGFfPhLmDontNA$jV6UkPlHVwP5CHy9EDSnwf73PbvI63nJ0P4iyXNmsHc',   '李小王',     'liwang@example.com',    2,    1);

INSERT INTO sys_user_role (user_id, role_id) VALUES
  (1, 1), (2, 2), (3, 3), (4, 2), (5, 3);

-- ---- 分类种子（家庭级） ----
INSERT INTO biz_category (id, family_id, name, sort) VALUES
  (1, 1, '厨房用品', 1),
  (2, 1, '衣物鞋帽', 2),
  (3, 1, '清洁用品', 3),
  (4, 2, '电子产品', 1),
  (5, 2, '工具五金', 2);

-- ---- 物品种子（家庭内共享；creator=提交人，owner=当前责任人） ----
INSERT INTO biz_item (id, family_id, name, quantity, category_id, remark, creator_id, owner_id) VALUES
  (1, 1, '不粘炒锅',     1, 1, '28cm，2024 年 618 入手，涂层完好',     2, 2),
  (2, 1, '冬季羽绒服',   2, 2, '一件黑色一件藏蓝，收纳袋在阳台柜顶',   2, 2),
  (3, 1, '备用钥匙',     1, NULL, '放在玄关抽屉第二格的小盒子里',      3, 3),
  (4, 2, '电动螺丝刀',   1, 5, '电池还行，充一次能用俩月',             4, 4),
  (5, 2, 'USB-C 数据线', 8, 4, '各种长短都有，大部分是快充线',         5, 4);

-- ---- 历史种子：每条物品补一条 CREATE 记录 ----
INSERT INTO biz_item_history (item_id, operator_id, operator_name, action, after_json) VALUES
  (1, 2, '张三', 'CREATE', JSON_OBJECT('name', '不粘炒锅', 'quantity', 1, 'remark', '28cm，2024 年 618 入手，涂层完好')),
  (2, 2, '张三', 'CREATE', JSON_OBJECT('name', '冬季羽绒服', 'quantity', 2, 'remark', '一件黑色一件藏蓝，收纳袋在阳台柜顶')),
  (3, 3, '张小妹', 'CREATE', JSON_OBJECT('name', '备用钥匙', 'quantity', 1, 'remark', '放在玄关抽屉第二格的小盒子里')),
  (4, 4, '李四', 'CREATE', JSON_OBJECT('name', '电动螺丝刀', 'quantity', 1, 'remark', '电池还行，充一次能用俩月')),
  (5, 5, '李小王', 'CREATE', JSON_OBJECT('name', 'USB-C 数据线', 'quantity', 8, 'remark', '各种长短都有，大部分是快充线'));
