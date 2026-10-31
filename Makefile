.PHONY: build build-backend build-frontend test test-backend test-frontend test-frontend-critical

FRONTEND_CRITICAL_VITEST := \
	src/i18n/__tests__/localesMessageCompile.spec.ts \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/keys.bulkUpdate.spec.ts \
	src/api/__tests__/redeem.spec.ts \
	src/components/common/__tests__/Select.ime.spec.ts \
	src/components/keys/__tests__/BulkEditKeysModal.spec.ts \
	src/views/user/__tests__/KeysView.spec.ts \
	src/views/user/__tests__/RedeemView.pagination.spec.ts \
	src/views/user/__tests__/ChannelStatusV1View.details.spec.ts \
	src/views/user/__tests__/ChannelStatusV1View.refresh.spec.ts \
	src/api/__tests__/admin.subscriptions.bulk.spec.ts \
	src/components/admin/subscription/__tests__/BulkSubscriptionActionDialog.spec.ts \
	src/components/admin/subscription/__tests__/bulkSubscriptionOperation.spec.ts \
	src/views/admin/__tests__/SubscriptionsView.bulkActions.spec.ts \
	src/views/admin/__tests__/SubscriptionsView.userUsageLink.spec.ts \
	src/components/account/__tests__/AccountPriorityCell.spec.ts \
	src/components/account/__tests__/UsageProgressBar.spec.ts \
	src/components/admin/group/__tests__/GroupRateMultipliersModal.edit.spec.ts \
	src/views/admin/__tests__/DashboardView.spec.ts \
	src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts \
	src/views/admin/ops/components/__tests__/OpsSettingsDialog.loading.spec.ts \
	src/views/admin/ops/components/__tests__/OpsAlertEventsCard.pagination.spec.ts \
	src/views/admin/ops/components/__tests__/OpsAlertRulesCard.duration.spec.ts \
	src/views/admin/ops/components/__tests__/OpsOpenAITokenStatsCard.spec.ts \
	src/api/__tests__/channelMonitorV2.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/components/payment/__tests__/paymentFlow.spec.ts \
	src/views/user/__tests__/ChannelStatusView.mode.spec.ts \
	src/views/user/__tests__/ChannelStatusV2View.ranking.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
	src/features/channel-monitor-v2/__tests__/MetricCell.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorZoom.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)
