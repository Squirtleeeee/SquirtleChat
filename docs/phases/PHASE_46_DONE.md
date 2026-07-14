# Phase 46 完成

## 需求分析
- 冒烟仅验证空搜索返回，未覆盖「有命中」路径
- README 阶段索引停在 Phase 40，与近期交付脱节

## 设计计划
- smoke：群详情成员数；DB 插入种子消息后搜索关键词命中；`around_seq` 非空
- README：功能列表补全 + Phase 33–46 索引表

## 交付
- `scripts/smoke-api.ps1`：group detail + 搜索命中
- `README.md`：功能与阶段索引

## 测试
```powershell
.\scripts\smoke-api.ps1
```

## 验证
- 冒烟 ALL PASSED（含 message search hit）
- README 可索引到 Phase 33–46

## 下阶段
Phase 47：移动端侧栏抽屉 / 窄屏适配
