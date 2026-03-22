#!/usr/bin/env bash
set -euo pipefail

# ─── Configuration ───────────────────────────────────────────────────────────
SERVER_PORT=8080
SERVER_URL="http://127.0.0.1:${SERVER_PORT}/"
DURATION="30s"
CONNECTIONS=(50 100 200)
ITERATIONS=3
WARMUP_DURATION="5s"
RESULTS_DIR="benchmark/results/$(date +%Y%m%d_%H%M%S)"

# ─── Colors ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✗]${NC} $*" >&2; }
header() { echo -e "\n${BOLD}${CYAN}═══ $* ═══${NC}\n"; }

# ─── Dependency checks ──────────────────────────────────────────────────────
check_deps() {
    local missing=()
    for cmd in wrk bombardier sarin gcc; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    if [[ ${#missing[@]} -gt 0 ]]; then
        err "Missing dependencies: ${missing[*]}"
        echo "Install them before running this benchmark."
        exit 1
    fi
    log "All dependencies found"
}

# ─── Build & manage the C server ────────────────────────────────────────────
build_server() {
    header "Building C HTTP server"
    gcc -O3 -o benchmark/server/bench-server benchmark/server/server.c
    log "Server built successfully"
}

start_server() {
    log "Starting server on port ${SERVER_PORT}..."
    benchmark/server/bench-server &
    SERVER_PID=$!
    sleep 1

    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        err "Server failed to start"
        exit 1
    fi
    log "Server running (PID: ${SERVER_PID})"
}

stop_server() {
    if [[ -n "${SERVER_PID:-}" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
        log "Server stopped"
    fi
}

trap stop_server EXIT

# ─── Resource monitoring ────────────────────────────────────────────────────
start_monitor() {
    local tool_name=$1
    local conns=$2
    local iter=$3
    local monitor_file="${RESULTS_DIR}/${tool_name}_c${conns}_i${iter}_resources.csv"

    echo "timestamp,cpu%,mem_kb" > "$monitor_file"

    (
        while true; do
            # Find the PID of the tool by name (exclude monitor itself)
            local pid
            pid=$(pgrep -x "$tool_name" 2>/dev/null | head -1) || true
            if [[ -n "$pid" ]]; then
                local stats
                stats=$(ps -p "$pid" -o %cpu=,%mem=,rss= 2>/dev/null) || true
                if [[ -n "$stats" ]]; then
                    local cpu mem_kb
                    cpu=$(echo "$stats" | awk '{print $1}')
                    mem_kb=$(echo "$stats" | awk '{print $3}')
                    echo "$(date +%s),$cpu,$mem_kb" >> "$monitor_file"
                fi
            fi
            sleep 0.5
        done
    ) &
    MONITOR_PID=$!
}

stop_monitor() {
    if [[ -n "${MONITOR_PID:-}" ]] && kill -0 "$MONITOR_PID" 2>/dev/null; then
        kill "$MONITOR_PID" 2>/dev/null || true
        wait "$MONITOR_PID" 2>/dev/null || true
    fi
}

# ─── Benchmark runners ─────────────────────────────────────────────────────
run_wrk() {
    local conns=$1
    local dur=$2
    local out_file=$3
    local threads=$((conns < 10 ? conns : 10))

    /usr/bin/time -v wrk -t"${threads}" -c"${conns}" -d"${dur}" "${SERVER_URL}" \
        2>"${out_file}.time" | tee "${out_file}.out"
}

run_bombardier() {
    local conns=$1
    local dur=$2
    local out_file=$3

    /usr/bin/time -v bombardier -c "${conns}" -d "${dur}" --print result "${SERVER_URL}" \
        2>"${out_file}.time" | tee "${out_file}.out"
}

run_sarin() {
    local conns=$1
    local dur=$2
    local out_file=$3

    /usr/bin/time -v sarin -U "${SERVER_URL}" -c "${conns}" -d "${dur}" -q \
        2>"${out_file}.time" | tee "${out_file}.out"
}

# ─── Warmup ──────────────────────────────────────────────────────────────────
warmup() {
    header "Warming up server"
    wrk -t4 -c50 -d"${WARMUP_DURATION}" "${SERVER_URL}" > /dev/null 2>&1
    log "Warmup complete"
    sleep 2
}

# ─── Extract peak memory from /usr/bin/time -v output ────────────────────────
extract_peak_mem() {
    local time_file=$1
    grep "Maximum resident set size" "$time_file" 2>/dev/null | awk '{print $NF}' || echo "N/A"
}

# ─── Extract total requests from tool output ─────────────────────────────────
extract_requests() {
    local tool=$1
    local out_file=$2

    case "$tool" in
        wrk)
            # wrk: "312513 requests in 2.10s, ..."
            grep "requests in" "$out_file" 2>/dev/null | awk '{print $1}' || echo "N/A"
            ;;
        bombardier)
            # bombardier: "1xx - 0, 2xx - 100000, 3xx - 0, 4xx - 0, 5xx - 0"
            # Sum all HTTP code counts
            grep -E "^\s+1xx" "$out_file" 2>/dev/null | \
                awk -F'[,-]' '{sum=0; for(i=1;i<=NF;i++){gsub(/[^0-9]/,"",$i); if($i+0>0) sum+=$i} print sum}' || echo "N/A"
            ;;
        sarin)
            # sarin table: "│ Total │ 1556177 │ ..."
            grep -i "total" "$out_file" 2>/dev/null | awk -F'│' '{gsub(/[[:space:]]/, "", $3); print $3}' || echo "N/A"
            ;;
    esac
}

extract_elapsed() {
    local time_file=$1
    grep "wall clock" "$time_file" 2>/dev/null | awk '{print $NF}' || echo "N/A"
}

extract_rps() {
    local tool=$1
    local out_file=$2

    case "$tool" in
        wrk)
            # wrk: "Requests/sec:  12345.67"
            grep "Requests/sec" "$out_file" 2>/dev/null | awk '{print $2}' || echo "N/A"
            ;;
        bombardier)
            # bombardier: "Reqs/sec  12345.67  ..."
            grep -i "reqs/sec" "$out_file" 2>/dev/null | awk '{print $2}' || echo "N/A"
            ;;
        sarin)
            # sarin doesn't output rps - calculate from total requests and duration
            local total
            total=$(extract_requests "sarin" "$out_file")
            if [[ "$total" != "N/A" && -n "$total" ]]; then
                local dur_secs
                dur_secs=$(echo "$DURATION" | sed 's/s$//')
                awk "BEGIN {printf \"%.2f\", $total / $dur_secs}"
            else
                echo "N/A"
            fi
            ;;
    esac
}

# ─── Print comparison table ──────────────────────────────────────────────────
print_table() {
    local title=$1
    local extract_fn=$2
    shift 2
    local columns=("$@")

    echo -e "${BOLD}${title}:${NC}"
    printf "%-12s" ""
    for col in "${columns[@]}"; do
        printf "%-18s" "$col"
    done
    echo ""

    local tools=("wrk" "bombardier" "sarin")
    for tool in "${tools[@]}"; do
        printf "%-12s" "$tool"
        for col in "${columns[@]}"; do
            local val
            val=$($extract_fn "$tool" "$col")
            printf "%-18s" "${val}"
        done
        echo ""
    done
    echo ""
}

# ─── Main ────────────────────────────────────────────────────────────────────
main() {
    header "HTTP Load Testing Tool Benchmark"
    echo "Tools:       wrk, bombardier, sarin"
    echo "Duration:    ${DURATION} per run"
    echo "Connections: ${CONNECTIONS[*]}"
    echo "Iterations:  ${ITERATIONS} per configuration"
    echo ""

    check_deps

    mkdir -p "${RESULTS_DIR}"
    log "Results will be saved to ${RESULTS_DIR}/"

    build_server
    start_server
    warmup

    local tools=("wrk" "bombardier" "sarin")

    for conns in "${CONNECTIONS[@]}"; do
        header "Testing with ${conns} connections"

        for tool in "${tools[@]}"; do
            echo -e "${BOLD}--- ${tool} (${conns} connections) ---${NC}"

            for iter in $(seq 1 "$ITERATIONS"); do
                local out_file="${RESULTS_DIR}/${tool}_c${conns}_i${iter}"
                echo -n "  Run ${iter}/${ITERATIONS}... "

                start_monitor "$tool" "$conns" "$iter"

                case "$tool" in
                    wrk)         run_wrk "$conns" "$DURATION" "$out_file" > /dev/null 2>&1 ;;
                    bombardier)  run_bombardier "$conns" "$DURATION" "$out_file" > /dev/null 2>&1 ;;
                    sarin)       run_sarin "$conns" "$DURATION" "$out_file" > /dev/null 2>&1 ;;
                esac

                stop_monitor

                local peak_mem rps elapsed
                peak_mem=$(extract_peak_mem "${out_file}.time")
                rps=$(extract_rps "$tool" "${out_file}.out")
                elapsed=$(extract_elapsed "${out_file}.time")
                echo -e "done (elapsed: ${elapsed}, rps: ${rps}, peak mem: ${peak_mem} KB)"

                sleep 2
            done
            echo ""
        done
    done

    # ─── Summary ─────────────────────────────────────────────────────────
    header "Summary"
    echo "Raw results saved to: ${RESULTS_DIR}/"
    echo ""
    echo "Files per run:"
    echo "  *.out       - tool stdout (throughput, latency stats)"
    echo "  *.time      - /usr/bin/time output (peak memory, CPU time)"
    echo "  *_resources.csv - sampled CPU/memory during run"
    echo ""

    local columns=()
    for conns in "${CONNECTIONS[@]}"; do
        columns+=("c=${conns}")
    done

    _get_rps() {
        local c=${2#c=}
        extract_rps "$1" "${RESULTS_DIR}/${1}_c${c}_i${ITERATIONS}.out"
    }

    _get_total() {
        local c=${2#c=}
        extract_requests "$1" "${RESULTS_DIR}/${1}_c${c}_i${ITERATIONS}.out"
    }

    _get_mem() {
        local c=${2#c=}
        extract_peak_mem "${RESULTS_DIR}/${1}_c${c}_i${ITERATIONS}.time"
    }

    _get_elapsed() {
        local c=${2#c=}
        extract_elapsed "${RESULTS_DIR}/${1}_c${c}_i${ITERATIONS}.time"
    }

    print_table "Requests/sec" _get_rps "${columns[@]}"
    print_table "Total Requests" _get_total "${columns[@]}"
    print_table "Elapsed Time" _get_elapsed "${columns[@]}"
    print_table "Peak Memory (KB)" _get_mem "${columns[@]}"

    log "Benchmark complete!"
    echo ""
    echo "To inspect individual results:"
    echo "  cat ${RESULTS_DIR}/wrk_c200_i1.out"
    echo "  cat ${RESULTS_DIR}/sarin_c200_i1.out"
    echo "  cat ${RESULTS_DIR}/bombardier_c200_i1.out"
}

main "$@"
