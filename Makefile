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
build: $(CMD)
	@echo cp ./$(CMD) "$(HOME)/Google\\ Drive/My\ Drive/irisctl/$(CMD)"

$(CMD): $(SRC)
	go build -o $(CMD) ./cmd/irisctl/...
	@ls -l $(CMD)

.PHONY: tags
tags:
	ctags $(SRC)

good:
	for i in $(SRC); do echo $$i; gofmt -w -s $$i; done
	golangci-lint run ./...

wc:
	wc -l $(SRC)

allmd:
	./$(CMD) meas all -a allmd
	jq . allmd | more

users: $(CMD)
	./$(CMD) users all > /tmp/__
	jq -r '.results[]|"\(.firstname) \(.lastname) \(.is_active) \(.is_superuser) \(.is_verified)"' /tmp/__
	rm -f /tmp/__

agents:
	./$(CMD).good agent all | tee /tmp/__
	jq -r '.results[] | "\(.uuid) \(.state) \(.parameters.hostname) \(.parameters.version)"' /tmp/__ | awk '{ printf("%s  %s  %-24s %s\n", $$1, $$2, $$3, $$4) }'
	#rm -f /tmp/__

whatis:
	-./$(CMD) users all | grep "${UUID}"
	-./$(CMD) agent all | grep "${UUID}"
	-./$(CMD) meaas all | grep "${UUID}"

examples:
	@echo ./$(CMD) analyze runs allmd
	@echo ./$(CMD) analyze -v runs allmd
	@echo ./$(CMD) analyze --after 2024-01-01 --state finished -t zeph runs allmd
	@echo ./$(CMD) analyze -v --after 2024-01-01 --state finished -t zeph runs allmd
	@echo ./$(CMD) analyze --before 2024-01-01 --state finished -t zeph runs allmd
	@echo ./$(CMD) analyze -v --after 2024-01-01 --state finished -t zeph runs allmd
	@echo ./$(CMD) analyze -v --after 2024-01-01 --state finished tags allmd
	@echo ./$(CMD) analyze -v --after 2024-01-01 --state finished hours allmd
	@echo ./$(CMD) analyze -v --after 2024-01-01 -t ipv6-hitlist.json runs allmd
	@echo ./$(CMD) analyze -v --after 2023-09-01 -t ipv6-hitlist.json hours allmd
	@echo ./$(CMD) analyze -v --after 2023-09-01 -t exhaustive-lip6.json hours allmd
