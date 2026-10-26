PY ?= .venv/bin/python

.PHONY: test test-go test-py run fmt

test: test-go test-py

test-go:
	go vet ./... && go test ./...

test-py:
	$(PY) -m pytest -q

run:
	go run ./go/cmd/searchd -corpus eval/sample_corpus.jsonl -addr :8080

fmt:
	gofmt -w go && ruff format py

beir:
	mkdir -p data && cd data && for d in fiqa scifact; do [ -d $$d ] || (curl -sSLO https://public.ukp.informatik.tu-darmstadt.de/thakur/BEIR/datasets/$$d.zip && unzip -q $$d.zip); done
	go build -o bin/bench ./go/cmd/bench
	for d in fiqa scifact; do ./bin/bench -beir data/$$d; done

pilot-report:
	$(PY) -m groundline.pilot verify && $(PY) -m groundline.pilot report

install:
	go install ./go/cmd/gl
