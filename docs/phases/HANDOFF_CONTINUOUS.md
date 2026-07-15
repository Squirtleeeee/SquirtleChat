# Continuous autonomous loop handoff

**Updated:** 2026-07-15 (~22:15 +08)

## Mandate

Fully automatic loop until user says 暂停. Currently paused for **user acceptance** of Phases 140–148.

## Completed (ready to accept)

140–148: mute/admin-only → nickname → invite links → slow mode → notify sync → member remarks → translate → welcome → dismissible notice

## Next (after acceptance)

Phase 149: Mark conversation unread **or** search filter by sender.

## How to run

- Backend: `.\scripts\start-backend.ps1` → :8080 / :8081
- Frontend: `.\scripts\start-frontend.ps1` → :5173
- Users: `test_a` / `test_b`，密码 `test1234`

## Suggested skills

- `squirtle-dev-cycle`
- `handoff`
