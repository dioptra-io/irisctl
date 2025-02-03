SRC=cmd/irisctl/main.go \
    internal/agents/agents.go \
    internal/analyze/analyze.go \
    internal/analyze/chart.go \
    internal/analyze/tables.go \
    internal/auth/auth.go \
    internal/check/check.go \
    internal/clickhouse/clickhouse.go \
    internal/common/common.go \
    internal/list/list.go \
    internal/maint/maint.go \
    internal/meas/meas.go \
    internal/status/status.go \
    internal/targets/targets.go \
    internal/users/users.go

CMD=irisctl

.PHONY: $(CMD)
$(CMD): $(SRC)
	go build -o $(CMD) ./cmd/irisctl/...

.PHONY: tags
tags:
	ctags $(SRC)

good:
	for i in $(SRC); do echo $$i; gofmt -w -s $$i; done
	golangci-lint run ./...

users: $(CMD)
	./$(CMD) users all | jq -r '.results[]|"\(.firstname) \(.lastname) \(.is_active) \(.is_superuser) \(.is_verified)"'
