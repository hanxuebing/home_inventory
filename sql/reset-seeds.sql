-- 清空种子数据（表结构保留）：init.sql 首次执行中途失败后的清理用
USE home_items;
DELETE FROM biz_item_history;
DELETE FROM biz_item;
DELETE FROM biz_category;
DELETE FROM sys_audit_log;
DELETE FROM sys_role_permission;
DELETE FROM sys_user_role;
DELETE FROM sys_role;
DELETE FROM sys_permission;
DELETE FROM sys_user;
DELETE FROM sys_family;
