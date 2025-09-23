# Roda benchmark específico

go test ./queue -bench=BenchmarkQueuer_Load

# Roda apenas benchmarks (sem testes unitários)

go test ./queue -bench=BenchmarkQueuer_Load -run=^$

# Roda todos os benchmarks

go test ./queue -bench=. -run=^$

# Com informações de memória

go test ./queue -bench=BenchmarkQueuer_Load -benchmem

# Com duração específica

go test ./queue -bench=BenchmarkQueuer_Load -benchtime=5s

# Com número específico de iterações

go test ./queue -bench=BenchmarkQueuer_Load -benchtime=100000x

# Roda multiple vezes para obter resultados mais estáveis

go test ./queue -bench=BenchmarkQueuer_Load -count=5

# Salva resultado em arquivo

go test ./queue -bench=BenchmarkQueuer_Load -benchmem > benchmark_results.txt
