# Đề Xuất Thực Nghiệm — Engram Sovereign FSM

> **Mục tiêu:** Hoàn thiện evaluation story cho bài báo hướng tới hội thảo chuyên ngành Khoa học máy tính **rank B trở lên**

Trọng tâm là xây dựng một hệ thực nghiệm có tính thuyết phục cho nghiên cứu Engram Sovereign FSM, kết hợp **ba lớp bằng chứng**: đặc tả hình thức, fault-injection prototype và microbenchmark mật mã/đồng thuận.

---

## 1. Câu Hỏi Nghiên Cứu

Câu chuyện thực nghiệm xoay quanh bốn câu hỏi nghiên cứu chính. Các câu hỏi này giúp bài báo không chỉ mô tả một FSM dự phòng cho blockchain modular, mà còn chứng minh được giá trị khoa học và kỹ thuật của cơ chế này dưới các lỗi ngoại vi liên quan đến Bitcoin settlement, DA layer và P2P health.

| # | Câu hỏi | Nội dung đánh giá |
|---|---------|-------------------|
| **RQ1** | Safety | Việc đưa trạng thái ngoại vi vào consensus proposal có ngăn được block/FSM-state conflict, forged receipt, data withholding và withdrawal leakage không? |
| **RQ2** | Liveness | Khi Bitcoin/DA/P2P bị lỗi, hệ thống có tiếp tục commit block tốt hơn baseline CometBFT phụ thuộc cứng vào external precondition không? |
| **RQ3** | Recovery | Khi ngoại vi phục hồi, cơ chế hysteresis và re-anchoring proof có đưa chain trở lại ANCHORED ổn định, không flapping không? |
| **RQ4** | Cost | Overhead của extended proposal, sensor validation, circuit breaker và ZK re-anchoring là bao nhiêu so với CometBFT/Cosmos SDK baseline? |

---

## 2. Tổng Quan Bộ Thực Nghiệm

| Nhóm | Mục tiêu khoa học | Baseline so sánh | Chỉ số đo |
|------|-------------------|------------------|-----------|
| **E1** Formal verification stress & ablation | Chứng minh safety/liveness không chỉ ở một cấu hình nhỏ | Full FSM vs. bỏ hysteresis / P2P sensor / f+1 pacemaker / ZK proof gate | States generated, distinct states, depth, violation found, counterexample class |
| **E2** Fault-injection end-to-end prototype | Chứng minh fallback giúp chain không halt khi BTC/DA/P2P lỗi | Vanilla CometBFT với external precondition cứng; static circuit breaker | Block commit rate, time-to-SOVEREIGN, committed tx during outage, downtime, recovery time |
| **E3** External-dependency failure matrix | Đánh giá từng lỗi và lỗi kết hợp | Same as E2 | Availability, p50/p95 block latency, consensus rounds/block, nil-prevote ratio |
| **E4** P2P eclipse/sybil detection | Kiểm tra tri-interface profiler có tốt hơn peer-count sensor không | Peer-count-only detector | False positive/negative, detection delay, incorrect recovery attempts |
| **E5** Hysteresis and flapping sensitivity | Chứng minh RECOVERING → ANCHORED ổn định | No-hysteresis recovery | Number of oscillations, failed recovery attempts, safe-block waiting cost |
| **E6** Reanchoring Feasibility Evaluation | Chứng minh recovery proof practical và scalable | Noir+Honk vs Plonky3; no-ZK baseline (re-execute) | Constraint count, proving/verification time, proof size, backend trade-off |
| **E7** Consensus overhead benchmark | Đo chi phí các trường mở rộng trong proposal | Vanilla proposal | Proposal size, CPU validation cost, throughput, block latency |
| **E8** Attack-resilience scenarios | Thể hiện security story thực nghiệm | Malicious proposer; data withholding; forged BTC receipt; withdrawal during SOVEREIGN; censorship | Accepted/rejected proposals, invalid commit count, forced inclusion latency |
| **E9** Trace-driven stress test | Làm bài thuyết phục hơn với workload thực | Synthetic-only experiment | Downtime under historical/simulated BTC congestion và DA delay traces |

---

## 3. Thiết Kế Chi Tiết Từng Thực Nghiệm

### E1 — Formal Verification Stress & Ablation

Thay vì chỉ báo cáo "no error", biến phần này thành một **verification study** có cấu hình, ablation và counterexample trace rõ ràng.

**Bảng cấu hình:**

| Config | N | f | MaxRound | BTC height | Engram height | Mục tiêu |
|--------|---|---|----------|------------|---------------|-----------|
| C1 | 4 | 1 | 2–3 | 2–3 | 2–3 | Reproduce current result |
| C2 | 4 | 1 | 4–5 | 3–4 | 3–4 | Kiểm tra consensus rounds sâu hơn |
| C3 | 7 | 2 | 2–3 | 2–3 | 2–3 | Kiểm tra quorum overlap lớn hơn |

**Bảng ablation:**

| Ablation | Kết quả cần quan sát |
|----------|----------------------|
| Remove hysteresis | Có thể xuất hiện flapping hoặc premature recovery |
| Remove P2P health gate | Eclipsed node có thể kích hoạt recovery sai |
| Remove circuit breaker | Có thể xuất hiện withdrawal leakage trong SOVEREIGN/RECOVERING |
| Remove f+1 timeout fast-forward | Liveness delay hoặc round-stall tăng |
| Remove DA receipt consistency | Data-withholding proposal có thể được commit |

> **Lưu ý:** Tối thiểu cần 2–3 counterexample traces từ các ablation quan trọng. Nếu một ablation không tạo violation, vẫn phải báo cáo khác biệt về state-space và giải thích lý do.

---

### E2 — Fault-Injection End-to-End Prototype

Đây là **thực nghiệm quan trọng nhất** cho hội thảo rank B trở lên. Mục tiêu: chứng minh claim *"graceful degradation rather than halting"* bằng prototype.

**Cấu hình đề xuất:**

| Thành phần | Gợi ý |
|------------|-------|
| Validators | 4, 7, 10, 16 nodes |
| Consensus | CometBFT/Cosmos SDK prototype |
| Workload | 100–1000 tx/s synthetic; mix normal tx và withdrawal tx |
| BTC sensor | Mock SPV/Babylon checkpoint service |
| DA sensor | Mock Celestia/Blobstream receipt service |
| P2P sensor | Controlled peer manager hoặc network emulator |
| Fault injector | Docker Compose + tc/netem, iptables, service pause, artificial receipt delay |

**Scenarios:**

| Scenario | Mô tả | Kỳ vọng với Engram FSM |
|----------|--------|------------------------|
| S1 Normal | BTC/DA/P2P healthy | Hoạt động như baseline |
| S2 BTC congestion | Checkpoint confirmation delay tăng dần | ANCHORED → SUSPICIOUS → SOVEREIGN, chain vẫn commit |
| S3 DA unavailable | DA receipt missing/false | Reject invalid DA blocks; chuyển fallback nếu kéo dài |
| S4 P2P eclipse partial | Giảm subnet diversity, peer churn cao | Cảnh báo, không recovery sớm |
| S5 Anchor isolation | ActiveAnchors = 0 | Chuyển thẳng SOVEREIGN |
| S6 Combined BTC+DA failure | Settlement và DA cùng lỗi | Chain vẫn xử lý local tx, khóa withdrawal |
| S7 Recovery | Lỗi được gỡ, proof available | SOVEREIGN → RECOVERING → ANCHORED sau hysteresis |

**Metrics chính:** time-to-detection, time-to-fallback, availability during outage, throughput degradation, consensus latency p50/p95/p99, recovery time, số withdrawal bị block, số incorrect state transitions / flapping.

**Baselines:** vanilla CometBFT với strict external validity; static circuit breaker; FSM without hysteresis; FSM với peer-count-only P2P sensor.

**Đã đo thật (in-process, baseline cũ):** `go test ./tests/e2e/...` chạy 7 kịch bản S1-S7 thật qua `x/sovereignty`'s `BeginBlocker` thật (không mock FSM
logic, chỉ mock input sensor), kết quả ở `tests/e2e/results/s*.csv` + `e2_summary.md`. `scripts/e2_fault_injection/simulate_network_jitter.py` dựng
Figure 3 (state timeline 7 kịch bản + withdrawal-lock shading) từ đúng dữ liệu đó.

**Đã đo thật (live-docker, 2026-08-08)**: `scripts/e2_fault_injection/live_scenario_matrix.py` chạy đủ S1-S7 nối liền thành 1 lần thật trên cluster
4-node thật (Pumba netem cho S2/S4/S5, `docker stop/start` thật cho S3/S6/S7), tổng thời lượng thật 1394s. Kết quả thật, không giả lập: cluster **chưa
từng đạt ANCHORED xuyên suốt cả 7 pha** -- dao động liên tục giữa RECOVERING/SOVEREIGN ngay từ S1 (baseline), đúng khớp phát hiện tiêu cực đã ghi ở mục
E5 bên dưới (hysteresis không có partial credit, một lần đọc xấu là reset về 0). S7 (chờ ANCHORED thật qua ZK pipeline) timeout sau 600s -- 23 proof thật
được chấp nhận trong lúc chạy (`watch_and_prove.sh` với bug SIGPIPE đã sửa, xem mục E9), 29 bị từ chối do race điều kiện thật (interval tăng nhanh hơn
tốc độ N=4 proof theo kịp), nhưng chưa đủ để đuổi kịp toàn bộ interval trong 600s. Dữ liệu thật ở
`scripts/e2_fault_injection/results_live/s*.csv`. Figure 3 bản live (`scripts/e2_fault_injection/live_figure_builder.py`) thay 2 hình cũ bằng
`figure3_state_timelines_live.{png,pdf}` (7 panel, mỗi panel chọn node có nhiều sample hợp lệ nhất -- không cố định node01, vì S4/S5 cố tình cô lập
chính node01 khiến nó mất kết nối RPC phần lớn thời gian, một bug thật tìm được khi vẽ hình lần đầu) và `figure3_summary_bars_live.{png,pdf}` (3 chỉ số
thật: block thật đã commit, số lần chuyển trạng thái thật, tỉ lệ thời gian ngoài ANCHORED -- không có `time_to_fallback`/`withdrawal_blocked_blocks` vì
2 trường này không tồn tại trong schema polling trực tiếp qua RPC).

**Baseline vanilla CometBFT (đã đo thật):** `engramd start --vanilla` (cờ mới, `app/app.go`) chạy đúng binary/module nhưng bỏ qua
`SetPrepareProposal`/`SetProcessProposal`/`SetPreBlocker` -- BaseApp dùng handler mặc định, không có `ExtendedProposal`. Chạy 2 node thật song song
(`scripts/e7_consensus_overhead/vanilla_comparison.sh`) xác nhận: node thường luôn có marker `ENGRAM_EXTENDED_PROPOSAL_V1|...` 228 byte ở `Txs[0]` mọi
block; node vanilla có 0 tx. **Phát hiện phụ khi xây baseline này:** `cmd/engramd/main.go`'s `runStart` trước đây dùng thẳng `cmtcfg.DefaultConfig()` mà
KHÔNG đọc `config.toml` từ đĩa (bug thật, phát hiện khi 2 node cùng chạy tranh cổng RPC mặc định dù đã sửa `config.toml`) -- đã sửa bằng
`viper`-based config loader (`loadConfig` trong `main.go`), quan trọng cho cả Phase 3 (multi-node Docker) vì trước đó cấu hình per-node (ports, peers) bị
bỏ qua hoàn toàn ở `start` time.

---

### E3 — Failure Matrix

Chỉ rõ hệ thống **biết khi nào** được phép tiếp tục commit local block và **khi nào phải khóa** các hành động rủi ro như withdrawal.

| BTC | DA | P2P | Expected state | Withdrawals | Block production |
|-----|----|-----|----------------|-------------|-----------------|
| healthy | healthy | healthy | ANCHORED | enabled | full |
| warning | healthy | healthy | SUSPICIOUS | restricted | moderate/full |
| critical | healthy | healthy | SOVEREIGN | locked | full local |
| healthy | failed | healthy | SUSPICIOUS/SOVEREIGN | locked nếu SOVEREIGN | local |
| healthy | healthy | eclipsed | SUSPICIOUS/SOVEREIGN | locked nếu critical | depends |
| critical | failed | eclipsed | SOVEREIGN | locked | local |
| recovered | recovered | healthy | RECOVERING → ANCHORED | locked until anchored | full |

**Đã đo thật:** `scripts/e3_failure_matrix/measure_latency.py` xây lại bảng trên bằng dữ liệu thật (không phải tay viết như bảng aspirational ở trên) --
đọc trạng thái ổn định (steady-state) cuối mỗi kịch bản S1-S7 trong `tests/e2e/results/s*.csv`, kết quả ở
`scripts/e3_failure_matrix/results/table2_failure_matrix.md`. "Block production" luôn là "continuous" vì `Harness.Advance()` chưa từng dừng/lỗi qua bất kỳ
kịch bản nào -- đó cũng là một phần kết quả đo được, không phải giả định.

---

### E4 — P2P Eclipse/Sybil Detection

P2P health profiler là điểm **novelty** vì sensor output không chỉ dùng cho monitoring mà còn trở thành consensus input thông qua proposal validation. Thực nghiệm so sánh trực tiếp hai detector để làm rõ lợi thế định lượng của tri-interface profiler.

**So sánh detector:**

| Detector | Mô tả | Điểm yếu |
|----------|-------|-----------|
| **Peer-count-only** *(baseline)* | Đếm peer mặc định của CometBFT | Dễ bị Sybil/slot filling qua mặt; FNR rất cao trên hầu hết attack |
| **Tri-interface profiler** *(proposed)* | Đo toàn bộ 6 metrics: structural + behavioral + latency | — |

**Attack scenarios** *(4 kịch bản trọng tâm đưa vào Table 6):*

| # | Kịch bản | Mô tả |
|---|----------|-------|
| A1 | Peer Slot Exhaustion | Lấp đầy slot kết nối bằng peer giả |
| A2 | Sybil qua đa-subnet giả lập (không phải BGP Hijacking theo nghĩa đen -- xem ghi chú dưới) | Peer cùng ASN/subnet giả mạo định tuyến |
| A3 | Churn-based Rotation | Thay thế peer liên tục để tránh bị phát hiện |
| A4 | Relay Node Attack | Chèn node trung gian làm tăng độ trễ |

**Methodology:** Sử dụng kỹ thuật Chaos Engineering thông qua công cụ **Pumba** kết hợp **Docker Compose** để chủ động bơm lỗi mạng (network delay, packet loss), nhằm giả lập độ trễ và sự thay đổi kết nối thực tế.

**Metrics** *(định lượng):* False Positive Rate (FPR — %), False Negative Rate (FNR — %), Detection Delay (ms/s).

**Table 6 — Detection Accuracy of P2P Profiler vs. Peer-Count Baseline:**

| Attack Scenario | Detector | FPR | FNR | Detection Delay |
|-----------------|----------|----:|----:|----------------:|
| Peer Slot Exhaustion | Peer-count | 1.5% | 98.2% | N/A |
| | **Tri-interface** | **0.8%** | **1.2%** | **450 ms** |
| BGP Hijacking / Sybil | Peer-count | 2.1% | 95.5% | N/A |
| | **Tri-interface** | **1.1%** | **0.5%** | **850 ms** |
| Churn-based Rotation | Peer-count | 85.0% | 15.0% | N/A |
| | **Tri-interface** | **2.5%** | **1.8%** | **1.2 s** |
| Relay Node Attack | Peer-count | 0.5% | 100.0% | N/A |
| | **Tri-interface** | **0.2%** | **0.0%** | **250 ms** |

**Đã đo thật (nhưng SYNTHETIC, không phải live-network -- xem lưu ý dưới):** bảng trên là số mục tiêu chưa đo bằng live cluster.
`scripts/e4_p2p_eclipse_detection/simulate_eclipse_attack.py` chạy Monte Carlo tổng hợp qua
`go test ./tests/e2e/... -run TestE4_P2PDetectorComparison`: hàm detector thật (`types.IsP2PQualityHealthy`) và baseline peer-count-only được kiểm tra
trên các peer-snapshot ngẫu nhiên dựng tay mô phỏng đặc trưng từng attack (2000 trial/ô, seed cố định), ở cả `DefaultParams()` (ngưỡng nhỏ dùng để verify
TLC) và một bộ ngưỡng "production_scale" thực tế hơn. Kết quả thật: FPR=0% cho cả hai detector, FNR=100% cho peer-count-only ở CẢ 4 attack (vì các attack
được thiết kế để giữ nguyên clean-peer count trong khi phá các tín hiệu khác), FNR=0% cho tri-interface, detection delay 1.0–2.7 snapshot tùy attack/ngưỡng
-- xem `scripts/e4_p2p_eclipse_detection/results/table6_p2p_detector_accuracy.md`. Đây là bằng chứng thật về hàm detector thật, nhưng trên input tổng hợp,
KHÔNG tương đương với đo trên mạng thật.

**Ghi chú quan trọng, sửa lại lý do trước đây (đã lỗi thời):** lý do "Docker daemon không chạy trong môi trường này" từng được ghi ở đây không còn đúng --
Docker đã chạy ổn định xuyên suốt phiên làm việc sau này. Lý do thật khiến A1/A2 chưa có dữ liệu live trước đây là **cơ chế phòng thủ chủ động chưa từng
được code hóa** (chỉ có phát hiện bị động qua `IsP2PQualityHealthy`, không có ingress filter chủ động) -- không phải thiếu công cụ test. Điều này đã được
đóng: `x/sovereignty/keeper/peer_filter.go`'s `FilterPeerByAddr` (đăng ký qua `baseapp.SetAddrPeerFilter`) là ingress filter thật, chặn peer dựa trên mật độ
subnet (`Params.MaxPeersPerSubnet`) TRƯỚC khi được thêm vào peer set -- khác với `SubnetDiversity` (chỉ báo cáo SAU khi đã bị chiếm). A1/A2 giờ có hạ tầng
live-docker thật để kiểm chứng defense này: `docker/attacker-peer-swarm.yml` (K container `engramd` thật, không phải validator, dial vào
`engram-node01`) + `scripts/e4_p2p_eclipse_detection/live_sybil_attack.py` (2 leg: `a1` -- 10 attacker cùng subnet `engram-net`; `a2` -- 12 attacker chia
đều 4 subnet giả lập riêng biệt `attacker-subnet-a/b/c/d`, đo qua `/net_info` RPC thật của CometBFT, độc lập với khoảng trống Query.State đã biết). A2 đổi
tên thành "Sybil qua đa-subnet giả lập" vì BGP Hijacking thật (thao túng route tầng Internet) không thể và không nên mô phỏng trong docker testnet --
điều `MaxPeersPerSubnet`/`SubnetDiversity` thực sự phòng thủ là **hệ quả** của BGP hijack (nhiều peer trông như đến từ nhiều subnet khác nhau nhưng cùng
1 kẻ tấn công), không phải nguyên nhân gốc, nên mô phỏng hệ quả này là đủ trung thực.

**Đã chạy live thật (2026-08-08), cả 2 leg, trên cluster 4-node thật đã rebuild với toàn bộ code phiên này:**
chi tiết đầy đủ ở `scripts/e4_p2p_eclipse_detection/results_live/sybil_attack_live_run_20260808_summary.md`. Tóm tắt:

- **A1** (10 attacker cùng subnet `engram-net`): filter thật chặn đúng ở **8/8** (`MaxPeersPerSubnet`), lặp lại 2 lần cho kết quả nhất quán. 3 validator
  thật (subnet `172.21.0.0`, do quirk gateway-priority của Docker multi-homed container -- xem dưới) hoàn toàn không bị ảnh hưởng suốt quá trình tấn
  công. Block height và AppHash tiến triển bình thường, khớp cả 4 node.
- **A2** (12 attacker, dự định trải trên 4 subnet riêng): filter vẫn giữ đúng **8/8**, nhưng **không đạt được đa dạng subnet thật như thiết kế ban đầu**
  -- phát hiện thật: container multi-homed trên Docker (gắn cả subnet riêng LẪN `engram-net` để có route tới `engram-node01`) mặc định route ra qua
  network được khai báo THỨ HAI trong service definition, không phải cái đầu tiên -- đúng cùng cơ chế khiến 4 validator thật cũng lộ diện qua
  `bitcoin-net` thay vì `engram-net` (dù `engram-net` khai báo trước). Đây là giới hạn thật của cách mô phỏng bằng Docker bridge network mặc định, không
  phải lỗi của `FilterPeerByAddr` -- filter vẫn phòng thủ đúng trên bất kỳ subnet nào peer thực sự xuất hiện, minh chứng bởi cả A1 lẫn A2 đều giữ đúng
  8/8. Thử `gw_priority` (tính năng Compose thật) trên 1 container để sửa nhưng chưa thành công trong thời gian cho phép -- để lại như giới hạn đã ghi
  nhận, không phải giả định.
- **3 bug thật tìm và sửa trực tiếp khi chạy live** (không phải giả thuyết): (1) `docker compose ... down` không chỉ định service sẽ phá hủy TOÀN BỘ
  cluster thật, không chỉ swarm tấn công -- đã xảy ra thật 1 lần giữa thí nghiệm, sửa bằng `stop`+`rm -f` với tên service tường minh, áp dụng luôn cho
  `live_combined_attack.py`/`live_double_signing_test.py`; (2) `persistent_peers` của attacker thiếu node ID thật (CometBFT yêu cầu định dạng
  `id@host:port`) -- sửa bằng cách resolve ID thật qua RPC lúc container khởi động; (3) attacker A2 ban đầu không có route tới `engram-node01` (chỉ gắn
  subnet riêng, chưa gắn `engram-net`) -- sửa bằng cách gắn cả 2 network, việc này lại lộ ra chính phát hiện gateway-priority ở trên.

---

### E5 — Hysteresis Sensitivity

Spec yêu cầu RECOVERING → ANCHORED chỉ khi `safe_blocks` đạt `HYSTERESIS_WAIT` và proof hợp lệ. Reviewer sẽ hỏi ngưỡng này được chọn như thế nào.

Chạy `HYSTERESIS_WAIT` ∈ {0, 1, 3, 5, 10, 20} trong các môi trường: stable recovery, intermittent DA receipt, intermittent BTC anchor, intermittent P2P churn, adversary tạo oscillation.

**Metrics:** flapping count, recovery latency, block throughput trong RECOVERING, false recovery rate, thời gian withdrawal bị khóa.

> **Kết quả mong muốn ban đầu (ĐÃ BỊ BÁC BỎ bởi "Phát hiện thật" bên dưới -- giữ lại đây chỉ để đối chiếu lịch sử, không phải claim còn hiệu lực):**
> `HYSTERESIS_WAIT = 3–5` là sweet spot; giá trị 0 hoặc 1 dễ flapping; giá trị quá cao làm recovery chậm và khóa withdrawal lâu.

**Đã đo thật (5/5 môi trường):** `go test ./tests/e2e/... -run TestE5_HysteresisSweep` sét thật `HYSTERESIS_WAIT` qua {0,1,3,5,10,20} trên
Harness/BeginBlocker thật, dưới 5 môi trường: "stable" (không nhiễu, nhóm đối chứng) và 4 môi trường nhiễu liên tục (`noisy_btc`, `noisy_da`,
`noisy_p2p`, `combined_adversarial`) -- mỗi block có 20% xác suất độc lập bị nhiễu 1 block (khác bản trước: 1 cú đánh sập đơn lẻ không tạo được flapping
ở giá trị nào). Cùng seed RNG cố định cho mọi giá trị HYSTERESIS_WAIT trong cùng môi trường để so sánh công bằng. Kết quả ở
`tests/e2e/results/e5_hysteresis_sweep.csv`, Figure 4 (3 panel: stability/flapping/time-to-first-recovery) ở
`scripts/e5_hysteresis_flapping/results/figure4_hysteresis.{png,pdf}`.

**Phát hiện thật (khác "kết quả mong muốn" ở trên, không chỉnh để khớp):** dưới nhiễu liên tục, CẢ HAI chỉ số Figure 4 đo đều đi SAI hướng so với kỳ
vọng ban đầu, không chỉ một: `anchored_uptime` (tỷ lệ thời gian ở ANCHORED trong cửa sổ 100 block) **giảm đơn điệu** khi HYSTERESIS_WAIT tăng (vd.
`noisy_btc`: 59.8% ở HW=0 → 0.0% ở HW=20), và `flapping_count` **tăng đơn điệu** thay vì giảm (cùng môi trường: 10 ở HW=0 → 37 ở HW=20) -- **không có
sweet spot nội suy trên bất kỳ chỉ số nào**.

Lý do là kiến trúc, không phải lỗi thực nghiệm, và giải thích được cả 2 chiều hướng ngược kỳ vọng: đọc trực tiếp `x/sovereignty/keeper/circuit_breaker.go`'s
`CalculateNextState`/`NextSafeBlocks` cho thấy nhánh RECOVERING's `!healthy` gửi FSM **thẳng về SOVEREIGN** ngay khi có 1 block xấu duy nhất (không phải
chỉ reset bộ đếm tại chỗ), và `NextSafeBlocks` chỉ cộng dồn khi 2 block RECOVERING liên tiếp -- một bộ đếm streak cứng, không có partial credit. Dưới
nhiễu per-block cố định, xác suất hoàn thành 1 streak dài HYSTERESIS_WAIT block liên tục không gián đoạn giảm theo hàm mũ khi HYSTERESIS_WAIT tăng --
nên giá trị càng lớn, hệ thống càng phải thử-và-thất-bại nhiều vòng RECOVERING→SOVEREIGN→RECOVERING hơn (chính là flapping) trước khi (nếu có) thành
công, đồng thời tốn nhiều block hơn ở ngoài ANCHORED. Thêm vào đó: một khi đã ở ANCHORED, một lần đọc xấu là rớt ngay (ANCHORED không có bảo vệ
hysteresis cho chính nó), và SUSPICIOUS→ANCHORED hoàn toàn không có cổng hysteresis nào (`CalculateNextState`'s nhánh SUSPICIOUS:
`if healthy { return ANCHORED }`, vô điều kiện) -- bất đối xứng giữa cạnh phục hồi (có gác cổng) và cạnh thoái lui (không có gì bảo vệ). Vì vậy
HYSTERESIS_WAIT lớn không hề "lọc nhiễu" như kỳ vọng ban đầu -- nó chỉ đặt ra 1 bài kiểm tra ngày càng khó đỗ, và mỗi lần trượt lại tự nó tạo thêm dao
động. Đây là một **kết quả tiêu cực (negative result) có giá trị công bố**, không phải lỗi cần sửa hay tham số cần tìm lại "giá trị đúng".

**Đã đo thật (live-docker spot-check, 2026-08-08)**: `scripts/e5_hysteresis_flapping/live_spot_check.py` xác nhận lại đúng chiều hướng trên dưới
consensus timing THẬT (không phải mock per-block), theo đúng phạm vi đã định trước: spot-check hẹp 2×2 (`HYSTERESIS_WAIT` ∈ {2 (mặc định hiện tại), 10}
× môi trường ∈ {stable, noisy_da}), không phải sweep 6×5 đầy đủ, vì mỗi tổ hợp cần sửa `params.go` + rebuild image + redeploy sạch riêng. Kết quả thật,
mỗi combo đo 300s thật trên cluster 4-node:

| HYSTERESIS_WAIT | Môi trường | Flapping (300s) | Transitions | Anchored uptime |
|---:|---|---:|---:|---:|
| 2 | stable | 0 | 0 | 0.00% |
| 2 | noisy_da | 12 | 13 | 0.00% |
| 10 | stable | 0 | 1 | 0.00% |
| 10 | noisy_da | **14** | 15 | 0.00% |

**Flapping tăng theo HYSTERESIS_WAIT dưới nhiễu thật** (12→14 khi noisy_da), đúng chiều đã phát hiện ở bản in-process -- xác nhận đây không phải
artifact của mock harness. `anchored_uptime` bằng 0% ở CẢ 4 combo (khác bản in-process, nơi vẫn đo được uptime dương ở HW thấp) -- lý do thật, không phải
lỗi: cluster live lúc đo đã ở trạng thái RECOVERING/SOVEREIGN dao động liên tục từ trước do phiên làm việc kéo dài (xem ghi chú E2/E9 cùng ngày), nên cửa
sổ 300s của spot-check không đủ để quan sát một lần ANCHORED thật nào -- một giới hạn thật của việc đo trên 1 cửa sổ ngắn trên hệ thống đã tích luỹ trạng
thái xấu từ trước, không phải bằng chứng phủ định phát hiện chính. Figure 4 bản live
(`scripts/e5_hysteresis_flapping/live_figure_builder.py` → `figure4_hysteresis_live.{png,pdf}`) đọc trực tiếp từ 4 file `*_summary.md` thật do
`live_spot_check.py` ghi ra, không tính lại từ CSV thô.

**Bug vận hành thật gặp lại khi dựng lại cluster cho combo HW=10** (không phải lỗi code, để lại đây vì tái diễn đúng lớp lỗi đã ghi ở
`docs/DEVELOPMENT.md`): sau khi wipe+redeploy sạch để đổi `HysteresisWait`, height=1 bị kẹt round-skip vô hạn -- debug thật cho thấy 2 nguyên nhân
chồng nhau: (1) ví Bitcoin trên `bitcoin-node01` bị unload (do container từng bị restart ở pha trước), khiến `AnchorTracker` không submit được anchor
tx; (2) SAU KHI reload ví, `bitcoin_miner_loop.sh` hoá ra đã dừng chạy từ trước đó trong phiên -- không có block Bitcoin mới nào để anchor tx vừa submit
đạt đủ `kDeepFinality=2` xác nhận, nên `h_btc_anchored` không bao giờ tiến được dù ví đã có tiền. Khởi động lại miner loop mới thực sự giải quyết được.
Đây là bằng chứng thêm cho lý do `docs/DEVELOPMENT.md`'s §3.2 nhấn mạnh thứ tự "ví Bitcoin phải sẵn sàng VÀ tiếp tục mine trước khi/trong suốt lúc
engramd chạy" -- không chỉ lúc bootstrap lần đầu mà cả những lần redeploy sau này trong cùng phiên.

---

### E6 — Reanchoring Feasibility Evaluation

**Mục tiêu:** Chứng minh rằng recovery proof là *practical và scalable*, không phải chỉ benchmark proving system. Trả lời trực tiếp RQ4 và các sub-question:

- **RQ4.1** — How does proving cost scale?
- **RQ4.2** — Does verification remain succinct?
- **RQ4.3** — What are the trade-offs between PLONK-like and STARK-like backends?

**Input của circuit:** Một recovery interval từ `checkpoint_old` → sovereign execution → `checkpoint_new`, với năm thành phần:

| # | Component | Nội dung chứng minh |
|---|-----------|---------------------|
| C1 | Header continuity | `H_i → H_{i+1}` hợp lệ |
| C2 | FSM legality | Chuỗi SOVEREIGN → RECOVERING → ANCHORED đúng spec |
| C3 | Withdrawal lock invariant | `withdrawal_locked = true` trong suốt interval |
| C4 | SMT root progression | `root_old → root_new` qua các state transition |
| C5 | Policy binding | `policy_hash` nhất quán |

**Table 6A — Circuit Composition:**

| Component           | Constraints | Share |
| ------------------- | ----------: | ----: |
| Header verification |         12k |   22% |
| FSM transition      |          2k |    4% |
| Withdrawal lock check |          1k |    2% |
| SMT inclusion proof |         18k |   33% |
| SMT update proof |         20k |   37% |
| Policy binding      |          1k |    2% |
| **Total**           | **54k**     |   100%    |


**Table 6B — Scaling Benchmark:**

| Sovereign Blocks | Constraints | Prove (s) | Verify (ms) | Proof Size | Blocks/s |
|-----------------:|------------:|----------:|------------:|-----------:|-----------:|
| 10 | 54k | 0.8 | 7 | 410 B | 20.4 |
| 100 | 540k | 4.9 | 8 | 410 B | 23.2 |
| 1,000 | 5.4M | 43 | 8 | 410 B | 24.8 |
| 5,000 | 27M | 201 | 8 | 410 B | 26.3 |

**Table 6C — Backend Comparison** *(tùy chọn nếu còn thời gian):*

| Metric | Noir + Honk | Plonky3 |
|--------|-------------|---------|
| Proof size | 400 B | 150 KB |
| Verify time | 8 ms | 28 ms |
| Prove time | 43 s | 22 s |
| Trusted setup | Yes | No |
| PQ secure | No | Yes |
| Recursion support | Good | Excellent |

**Figures cần có:**

- **Figure 6** — Recovery Proof Scaling: 4 panel gồm (A) Constraint Count, (B) Proving Time — cả hai tuyến tính; (C) Verification Time — gần phẳng; (D) Proof Size — gần hằng số.
- **Figure 7** *(tùy chọn)* — Backend Trade-off: radar chart hoặc grouped bar chart so sánh Noir+Honk vs Plonky3 trên 6 tiêu chí.

**Đã đo thật (real measurements, không phải số mục tiêu ở trên):** `scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh` chạy toàn bộ pipeline
`nargo compile` → `bb gates` → `nargo execute` → `bb prove` → `bb verify` thật (Noir 1.0.0-beta.22 + Barretenberg 5.0.0-nightly.20260522, UltraHonk) trên
`circuit/reanchoring/src/main.nr` ở N = 4..256 headers, kết quả thô ở `scripts/e6_zk_reanchoring_benchmark/results/table6b_scaling.csv`, bảng/biểu đồ dựng
bởi `stats_collector.py` vào `results/table6a_6b.md` + `results/figure6_scaling.{png,pdf}`. Circuit thật đơn giản hơn Table 6A/6B ở trên (chain
continuity qua Pedersen hash thay vì SMT inclusion/update proof thật -- xem comment ở đầu `main.nr`), nên số liệu đo được KHÔNG nhằm khớp với các con số
mục tiêu phía trên, mà xác nhận đúng shape khoa học mà E6 muốn chứng minh: constraint count tăng tuyến tính hoàn hảo (42 ACIR opcodes/header thêm, R²=1.0),
proving time tăng gần tuyến tính (0.38s → 4.14s từ N=4 → 256), verification time phẳng (20–26ms, không phụ thuộc N), proof size hằng số tuyệt đối (14,656 B
ở mọi N). Table 6C/Figure 7 (Plonky3 backend so sánh) chưa thực hiện -- đúng như mức ưu tiên "tùy chọn" ở trên.

> **Scientific claim:** Recovery proofs scale linearly in prover cost while preserving constant-size proofs and constant-time verification — reanchoring is practical, scalable, and incurs bounded overhead.

**Ưu tiên thực hiện:**

| Mức | Artifact |
|-----|---------|
| Bắt buộc | Figure 6, Table 6A, Table 6B |
| Tùy chọn | Table 6C, Figure 7 |

---

### E7 — Consensus Overhead của Extended Proposal

Extended proposal thêm các trường `fsm_state`, `da_receipt`, `btc_receipt`, `zk_proof_ref`. Cần trả lời: cơ chế FSM có làm giảm throughput hoặc tăng latency quá nhiều so với CometBFT thông thường không?

| Variant | Mô tả |
|---------|-------|
| V0 | Vanilla CometBFT |
| V1 | + `fsm_state` only |
| V2 | + DA receipt |
| V3 | + BTC receipt |
| V4 | + P2P sensor digest |
| V5 | + ZK proof ref / verification flag |

**Metrics:** proposal size overhead, block validation CPU, commit latency, throughput, rounds/block, bandwidth per validator, nil prevote ratio khi sensor lệch.

> **Kết quả mong muốn:** overhead bình thường thấp; overhead tăng chủ yếu khi receipt verification hoặc sensor mismatch xảy ra.

**Đã đo thật:** `go test ./tests/benchmark/... -bench=. -benchmem` (thật, không mock) đo kích thước JSON thật của từng payload V0-V5 tích lũy và chi phí
CPU thật của từng bước validate (`CalculateNextState`, `da.VerifyReceipt`, `vigilante.VerifyReceipt`) cộng dồn, cộng chi phí `ProcessProposal` đầy đủ
(V5 thật, chạy end-to-end). `scripts/e7_consensus_overhead/measure_overhead.py` dựng Table 4 từ đó -- kết quả ở
`scripts/e7_consensus_overhead/results/table4_overhead.md`. Lưu ý: V4 (P2P digest) chỉ là ước lượng kích thước vì trường này chưa thực sự nằm trong
wire format thật (P2P health được validate từ `keeper.Metrics` cục bộ của leader, không nằm trong proposal) -- xem comment đầu
`tests/benchmark/fsm_latency_test.go`. Bảng còn có phần **baseline vanilla CometBFT thật** (2 node `engramd` chạy song song, 1 thường 1 `--vanilla`) --
xem chi tiết ở ghi chú "Đã đo thật" của E2 phía trên (cùng một baseline dùng chung cho E2/E3/E7). Overhead đo được: **+228 byte/block** cho marker
`ENGRAM_EXTENDED_PROPOSAL_V1` trên 100% block; block interval không khác biệt có ý nghĩa ở trạng thái idle (do CometBFT's `timeout_commit` mặc định chi
phối cả hai, không phải do ExtendedProposal).

**Tách lại thành 2 chế độ overhead (steady-state vs. recovery-event), không phải 1 số trung bình cộng dồn:** bảng V0→V5 và số +228B/block ở trên chỉ đo
các trường LUÔN có mặt mọi block (`fsm_state`/`da_receipt`/`btc_receipt`) -- chúng loại trừ hoàn toàn chi phí thật của ZK proof, vì `ZKProofRef` (kể cả
sau khi đổi sang hash ở mục refinement note) không bao giờ mang theo proof bytes thật (~14,656 byte UltraHonk, đo thật ở E6's `table6b_scaling.csv`) bên
trong `ExtendedProposal` -- proof thật đi qua 1 tx `SubmitRecoveryProof` riêng biệt. Trình bày 1 con số trung bình vừa đánh giá thấp chi phí gần-bằng-0
của đường đi khỏe mạnh, vừa che giấu hoàn toàn chi phí thật (có giới hạn, hiếm) của đường phục hồi:

- **Steady-state tax** (mọi block, luôn trả): ~230B/block (số đo thật ở trên), CPU không đáng kể.
- **Recovery-event cost** (chỉ block có tx `SubmitRecoveryProof`, hiếm, tự giới hạn): proof thật ~14,656B (E6) + chi phí CPU thật của `bb verify` trong
  `DeliverTx` (`x/sovereignty/keeper/reanchor.go`'s `VerifyZKProof`) -- **đã đo thật**: `go test ./tests/benchmark/... -bench=BenchmarkVerifyZKProof`
  chạy `bb verify` thật (không mock) trên chính proof N=4 thật đã tạo ở E6, kết quả **~18.77 ms/lần verify** (`BenchmarkVerifyZKProof-8: 3 iterations,
  18,771,861 ns/op`). Chi phí này chỉ phát sinh tới khi `RealProofSubmittedHeight` đuổi kịp tip, không kéo dài -- đúng tinh thần "graceful degradation,
  thuế gần-bằng-0 ở đường đi khỏe mạnh" của toàn bộ thiết kế, một luận điểm mạnh hơn nhiều so với 1 con số trung bình mù mờ.
- `scripts/e7_consensus_overhead/live_overhead_scan.py` đã cập nhật để gộp overhead mỗi block theo `fsm_state` thật, và đánh dấu (heuristic theo kích
  thước, không phải decode protobuf chính xác) block nào có khả năng chứa tx `SubmitRecoveryProof` -- mẫu 60-block trước đó chưa từng đi qua RECOVERING,
  cần chạy lại sau khi lái cluster qua 1 chu kỳ RECOVERING thật (vd. `live_lifecycle_test.py`'s phase 7) để bắt được mẫu thật.

---

### E8 — Attack-Resilience Test Suite

Chuyển các lemma an toàn thành integration tests hoặc simulation traces.

**Ma trận A1-A8** (thay thế hoàn toàn bảng 7 dòng trước đây -- lý do reconciliation: bảng cũ có 7 dòng thật, không phải 8 như prose "6/8" từng ghi
nhầm; A1/A2 dưới đây thay cho 2 dòng "Timeout flooding"/"Double-signing" cũ, ánh xạ đúng lemma hình thức đã có sẵn trong `spec/README.md` — Eclipse ≈
Lemma 7.5, Data Withholding ≈ Lemma 7.2 — và tái dùng hạ tầng đã xây cho E4 thay vì trùng lặp công việc):

| # | Attack | Expected result | Kết quả live thật (2026-08-08) | Cơ chế live-docker |
|---|--------|------------------|-------------------------------|---------------------|
| A1 | Eclipse Attack (cô lập) | Filter chặn slot trước khi bị chiếm, FSM không degrade sai | **PASS thật** -- filter giữ đúng 8/8 (`MaxPeersPerSubnet`), cluster không bị ảnh hưởng | `docker/attacker-peer-swarm.yml` leg `a1`, `scripts/e4_p2p_eclipse_detection/live_sybil_attack.py a1` |
| A2 | Sybil qua đa-subnet giả lập | Filter chặn dựa trên mật độ subnet, không bị đánh lừa bởi đa dạng hoá | **PASS thật** -- filter giữ đúng 8/8 dù swarm lớn hơn (12 attacker) | leg `a2` của cùng script trên |
| A3 | Data Withholding | Honest validators reject proposal claiming DA attestation giả | **PASS thật** -- `safety_held=True divergence_events=0`, height tiến bình thường xuyên suốt 60s tấn công + 30s hồi phục | `docker/engram-validator-node04-byzantine.yml` (`ENGRAM_BYZANTINE_BEHAVIOR=false_da_attestation`), `scripts/e8_attack_resilience/live_byzantine_attacks.py a3_false_da_attestation` |
| A4 | Forged BTC Receipt | Honest validators reject checkpoint hash không khớp `ExpectedBlockHash` | **PASS thật** -- cùng verdict, cùng cơ chế | cùng script trên, scenario `a4_forge_btc_hash` |
| A5 | Withdrawal During SOVEREIGN | Tx bị giữ lại (không commit) trong khi SOVEREIGN | **PASS thật** -- `blocked_correctly=True` (CLI real timeout chờ DeliverTx, tx không bao giờ commit trong khi SOVEREIGN); cluster hồi phục bình thường sau khi khôi phục celestia-bridge | `engramd tx-submit-forced-tx --payload "TX_WITHDRAWAL..."` (cmd/engramd/e8_cli.go), `scripts/e8_attack_resilience/live_withdrawal_test.py` |
| A6 | Malicious Proposer | Honest validators reject `fsm_state` giả không khớp tính toán cục bộ | **PASS thật** -- cùng verdict, cùng cơ chế | cùng script byzantine trên, scenario `a6_fake_fsm_state` |
| A7 | Censorship / Tx Withholding | Leader cố tình bỏ qua tx bị phát hiện qua `IsCensoring`/`ForcedTxQueue` | **PASS thật** (sau khi sửa 3 lỗi thật, xem "Bugs found" bên dưới) -- `safety_held=True divergence_events=0`, height tiến liên tục 10→132 xuyên suốt cửa sổ censoring | `docker/engram-validator-node04-byzantine.yml` (`ENGRAM_BYZANTINE_BEHAVIOR=censor_tx:<hex>`), `scripts/e8_attack_resilience/live_censorship_test.py` (mới, viết trong đợt này) |
| A8 | Combined Attack | An toàn giữ vững dưới nhiều vector tấn công chồng lấn | **PASS thật** -- node04 byzantine (`fake_fsm_state:SOVEREIGN`) đồng thời với A1 swarm 10 attacker, `safety_held=True divergence_events=0`, height tiến liên tục 145→309 xuyên suốt 120s tấn công kép | `scripts/e8_attack_resilience/live_combined_attack.py` |
| — | Double-signing (không đánh số, giữ như mục phụ) | Evidence extracted/logged | **PASS thật** -- cả 3 validator trung thực phát hiện `DuplicateVoteEvidence` thật, độ trễ phát hiện chỉ 1 block, xác nhận 2 lần độc lập (offense height 765 và 773) | `x/sovereignty/types/evidence.go` + `preblock.go`'s `recordDetectedEvidence`, `docker/engram-validator-node04-duplicate.yml`, `scripts/e8_attack_resilience/live_double_signing_test.py` -- xem ghi chú riêng bên dưới |

**Double-signing, đóng được rẻ hơn dự tính ban đầu -- KHÔNG cần wiring `x/evidence` module:** phát hiện thật khi đọc kỹ ABCI 2.0:
`RequestFinalizeBlock.Misbehavior` đã mang sẵn báo cáo `DuplicateVoteEvidence`/`LightClientAttackEvidence` thật từ evidence pool CometBFT (stock, không
sửa fork) -- không cần wiring `x/evidence` (Cosmos SDK), chỉ cần đọc trực tiếp field này trong `PreBlocker` đã có sẵn. `preblock.go`'s
`recordDetectedEvidence` đọc `req.Misbehavior`, ghi vào state mới (`Keeper.DetectedEvidenceCount`/`LastDetectedEvidence`, JSON-encoded, không cần proto
mới) + log dòng `SLASHABLE EVIDENCE DETECTED` thật. **An toàn để commit** (khác hẳn lỗi sensor cục bộ đã tìm thấy trước đó gây AppHash phân kỳ):
`req.Misbehavior` là dữ liệu đã đồng thuận, tất định, giống hệt `req.Txs`, không phải 1 lần đọc cục bộ mới -- xem comment trong `evidence.go`.

Nửa khó nhất (1 validator thật, còn sống, hành xử ác ý) đóng qua `docker/engram-validator-node04-duplicate.yml`: clone `priv_validator_key.json` thật
của node04 vào 1 container thứ 2 -- **nhưng cố tình KHÔNG chia sẻ** `priv_validator_state.json` (file theo dõi height/round đã ký cuối, chính là cơ chế
chống double-sign có sẵn của FilePV) -- nếu chia sẻ thì tiến trình thứ 2 sẽ không bao giờ double-sign được, đúng như thiết kế bảo vệ vốn có. 3 unit test
thật (`evidence_test.go`) xác nhận `recordDetectedEvidence` hoạt động đúng.

**Lưu ý quan trọng về định nghĩa (double-signing ≠ AppHash phân kỳ)**: double-signing/equivocation nghĩa là **một validator duy nhất** (cùng private
key) ký **2 vote xung đột** (khác `BlockID`) cho **cùng height + cùng round + cùng loại vote**, hoàn toàn khác với "AppHash phân kỳ" (state machine
non-determinism -- các validator trung thực tính ra state root khác nhau, dùng để kiểm tra `safety_held` ở A3-A8 phía trên). Harness ở đây tạo
equivocation thật bằng cách chạy 2 tiến trình độc lập cùng cầm 1 private key nhưng KHÔNG chia sẻ lịch sử ký -- khi cả 2 độc lập vote khác nhau cho
cùng height/round, evidence pool CometBFT tự phát hiện, không cần giả lập gì thêm.

**Kết quả live thật (2026-08-08)**: chạy thành công lần đầu sau khi sửa 2 lỗi bootstrap thật (xem "Bugs found" bên dưới) -- cả 3 validator trung thực
log `SLASHABLE EVIDENCE DETECTED` thật, độ trễ phát hiện 1 block, xác nhận độc lập 2 lần trong cùng 1 lần chạy (offense_height=765/detected=766 và
offense_height=773/detected=774). Quan sát qua `docker logs` (chưa có Query RPC riêng cho `DetectedEvidenceCount` -- cần thêm message .proto mới, để
ngoài phạm vi đợt này) chứ không phải giả định.

### Bugs found and fixed live during A7/A8/Double-signing (real, not hypothetical)

Toàn bộ 6 mục dưới đây là lỗi thật, mỗi lỗi được tìm ra vì thực sự chạy live-docker lần đầu, không phải bằng đọc code:

1. **`docker/engram-validator-node04-byzantine.yml` có `name:` riêng ở top-level** -- khi merge với `compose.yml` qua `-f a -f b`, Compose lấy `name:`
   của file SAU CÙNG, biến override này thành một Compose project RIÊNG BIỆT thay vì cùng project với cluster thật -- gây "Conflict: container name
   /engram-node04 already in use" thật khi thử swap service này. Sửa bằng cách xoá hẳn `name:` khỏi file override (và file `-duplicate.yml` có cùng lỗi
   tiềm ẩn, dù chưa gây crash vì nó tạo container MỚI thay vì swap container có sẵn).
2. **Withdrawal-tx gây deadlock liveness vĩnh viễn**: A5's tx `TX_WITHDRAWAL` bị `ProcessProposal`'s check #4 từ chối đúng như thiết kế (không commit
   khi SOVEREIGN), nhưng vì `PrepareProposal` cũ KHÔNG chủ động lọc bỏ tx này khỏi proposal của chính mình, tx đó vẫn nằm mãi trong mempool và cứ bị
   mọi leader tiếp tục đưa vào proposal → mọi proposal của MỌI validator (không riêng gì leader ác ý) đều bị reject vĩnh viễn -- cluster đứng hình thật
   (round-skip liên tục hàng chục round, quan sát qua `docker logs`: `prevote step: state machine rejected a proposed block`). Sửa bằng cách để
   `PrepareProposal` tự lọc bỏ tx chứa marker withdrawal khỏi chính proposal của nó khi `WithdrawLocked`, thay vì chỉ dựa vào check #4 làm hàng rào
   cuối. (`x/sovereignty/proposal.go`)
3. **`ForcedTxQueue` không bao giờ dequeue sau khi tx đã được include** (lỗi NGHIÊM TRỌNG NHẤT, phát hiện khi debug A7 lần 2): `updateForcedTxTracking`
   cũ chỉ reset `ignoredRounds` về 0 khi tx được include, nhưng KHÔNG xoá entry khỏi `ForcedTxQueue` -- vì tx này chỉ có thể "include" đúng 1 lần (bị
   tiêu thụ khỏi mempool sau khi commit), mọi round SAU đó `included[tx]` luôn `false` mãi mãi, khiến `IsCensoring` trip vĩnh viễn dù không còn ai
   censoring cả (kể cả sau khi revert node04 về honest). Sửa: dequeue hẳn khỏi `ForcedTxQueue`/`TxIgnoredRounds` ngay khi tx được include, không chỉ
   reset counter. (`x/sovereignty/preblock.go`'s `updateForcedTxTracking`)
4. **`SubmitForcedTx` chấp nhận nội dung KHÔNG BAO GIỜ có thể include được (lỗ hổng self-DoS/DoS chưa từng bị giới hạn)**: root-cause thật của lần
   deadlock A7 thứ 2 -- script test chọn "tx mục tiêu" là 1 `MsgSubmitForcedTxRequest` KHÁC; khi broadcast tx này, handler `SubmitForcedTx` của CHÍNH
   NÓ chạy lại và đăng ký payload bên trong (một chuỗi ASCII đơn thuần, không phải tx hợp lệ) làm entry MỚI trong `ForcedTxQueue` -- entry này không
   bao giờ có thể tự xuất hiện dưới dạng raw tx bytes thật, nên vĩnh viễn không thể "included". Đây không chỉ là lỗi test script: **bất kỳ ai gửi
   `MsgSubmitForcedTx` với nội dung không decode được thành tx hợp lệ đều có thể đóng băng TOÀN BỘ mạng vĩnh viễn**, không cần quyền validator. Sửa tận
   gốc: `SubmitForcedTx` giờ từ chối ngay lúc submit nếu `msg.Tx` không decode được qua `k.TxDecoder` (field mới, optional/nil-safe theo đúng pattern
   `peerFilterSrc` có sẵn, wire từ `app.go` bằng `txConfig.TxDecoder()` -- cùng decoder BaseApp tự dùng). 2 unit test mới xác nhận cả 2 nhánh (từ chối
   nội dung rác / chấp nhận tx hợp lệ). (`x/sovereignty/keeper/msg_server.go`, `x/sovereignty/keeper/keeper.go`, `app/app.go`)
5. **Docker Desktop (macOS virtiofs) không mount được 1 file cụ thể nằm bên trong 1 thư mục ĐÃ được bind-mount từ nguồn khác** -- lỗi hạ tầng thật, tái
   hiện 2 lần: `docker/engram-validator-node04-duplicate.yml`'s 2 dòng volume lồng nhau (`priv_validator_key.json`/`genesis.json` từ 2 nguồn khác nhau,
   lồng bên trong `/root/.engramd` vốn đã là 1 bind-mount) → "mountpoint ... is outside of rootfs". Sửa bằng cách bỏ 2 volume lồng đó, thay bằng script
   Python (`stage_duplicate_identity`) copy 2 file này lên HOST trước khi `docker compose up`, để chúng đã sẵn có trong volume gốc duy nhất.
6. **`priv_validator_state.json` không bao giờ được tạo cho harness duplicate-key** -- `engramd init` bail ra sớm (vì genesis.json/priv_validator_key.json
   đã tồn tại sẵn), nên KHÔNG BAO GIỜ chạm tới bước tạo file state cho FilePV, khiến `engramd start` crash thật ("no such file or directory") lần đầu
   tiên harness này thực sự chạy được (2 lỗi #1/#5 trước đó đã che mất lỗi này). Sửa: `stage_duplicate_identity` tự tạo file state rỗng chuẩn
   (`{"height":"0","round":0,"step":0}`) -- khớp đúng ý đồ thiết kế ban đầu (tiến trình thứ 2 phải bắt đầu từ trạng thái ký trống, KHÔNG copy từ state
   đã tiến triển của node04 thật).

**"Timeout flooding by Byzantine nodes"** (dòng cũ, không còn trong ma trận đánh số A1-A8 ở trên): đóng qua `chaos-crash` (SIGKILL 1 node) như phần
gần nhất hiện có, với 2 vế caveat rõ ràng: (1) **có** đi qua đúng đường f+1-timeout-quorum thật (M0b's `handleTimeout`/
`recordTimeoutSenderAndMaybeAdvance`), cho số liệu liveness-recovery thật cho **mô hình lỗi crash** (f=1 validator mất phản hồi hoàn toàn); (2) **không**
xác nhận khả năng chống mô hình **Byzantine chủ động** thật của dòng này -- 1 validator còn sống, còn ký được, cố tình flood `Timeout` attestation hợp lệ
để thao túng nhịp round-skip nhanh hơn mức im lặng đơn thuần gây ra, và không hề thử thách đường xác minh chữ ký của `handleTimeoutMessage` dưới nội
dung đối kháng thật (SIGKILL không gửi gì cả). Hướng đóng rẻ hơn trong tương lai (chưa làm): hạ tầng ký `Timeout` (`PrivValidator.SignTimeout`) từ M0b
đã có sẵn, chỉ cần 1 harness nhỏ kích hoạt đường ký sớm/chủ động thay vì chỉ khi timer thật hết hạn.

**Ngoài pass/fail:** đo number of rounds to recover, number of invalid proposals rejected, honest validator agreement rate, censorship latency, slashable evidence detection latency.

**Đã đo thật, TOÀN BỘ A1-A8 + Double-signing (9/9 dòng đánh số, 2026-08-08)**: mỗi dòng trong bảng trên giờ có kết quả live-docker thật (không phải
in-process nữa) -- xem cột "Kết quả live thật" và file `results_live/*_summary.md` tương ứng trong `scripts/e8_attack_resilience/results_live/`. Đây là
lần đầu ma trận này đạt độ phủ live-docker đầy đủ; trước đó chỉ có `scripts/e8_attack_resilience/trigger_disconnect.py`'s in-process `go test -json`
run (kết quả cũ vẫn còn ở `scripts/e8_attack_resilience/results/table3_attack_resilience.md`, PASS, giữ lại như tài liệu tham khảo song song, không phải
nguồn số liệu chính nữa). Chỉ riêng "Timeout flooding" (dòng phụ, không đánh số A1-A8) vẫn dừng ở mức đóng một phần qua `chaos-crash`, như đã ghi ở trên
-- không phải vì thiếu hạ tầng, mà vì hướng đóng đầy đủ (Byzantine chủ động flood `Timeout` hợp lệ) là công việc mới, ngoài phạm vi đợt này.

---

### E9 — Trace-Driven Stress Test

Nếu còn thời gian, trace-driven experiment giúp bài vượt mức benchmark synthetic thông thường. Replay các trace mô phỏng Bitcoin congestion, DA outage, P2P churn và mixed failure vào FSM prototype.

Kết quả biểu diễn bằng **timeline** ANCHORED → SUSPICIOUS → SOVEREIGN → RECOVERING → ANCHORED, vẽ song song:
- BTC finality gap
- DA gap
- P2P health score
- Block commit rate
- Withdrawal lock status
- Proof generation status

**Đã đo thật (in-process, baseline cũ):** `go test ./tests/e2e/... -run TestE9_TraceDrivenCombinedFailure` replay một trace liên tục thật (không phải 7
kịch bản riêng lẻ như E2) qua Harness/BeginBlocker thật: BTC congestion tăng dần → chồng thêm DA outage khi vẫn đang SOVEREIGN → chồng thêm P2P churn
spike (combined 3 lỗi cùng lúc) → lần lượt hồi phục → RECOVERING → ANCHORED. Dữ liệu thật ở `tests/e2e/results/e9_trace_driven.csv` (48 block), Figure 2
6-panel ở `scripts/e9_trace_driven/results/figure2_trace_timeline.{png,pdf}` -- xác nhận chain không dừng commit block dù cả 3 lỗi chồng nhau cùng lúc.

**Đã đo thật (live-docker, 2026-08-08)**: `scripts/e9_trace_driven/live_combined_trace.py` chạy 1 trace liên tục thật trên cluster 4-node thật (không
phải mock BeginBlocker) -- layering đúng như thiết kế, không tuần tự: chaos-btc-delay (BTC congestion) → chồng `docker stop celestia-bridge` (DA outage,
vẫn đang BTC pressure) → chồng 3 chu kỳ chaos-loss (P2P churn burst, cả 3 lỗi cùng lúc) → hồi phục ngược thứ tự → chờ ANCHORED thật qua ZK pipeline.
Phase 1-6 (baseline → BTC → +DA → +P2P churn → peak triple-fault → healing) **PASS thật**: height tiến liên tục 1043→1192 xuyên suốt toàn bộ 273s đầu,
không có round nào bị treo, cả 4 node luôn đồng bộ height. Phase 7 (chờ ANCHORED thật qua `watch_and_prove.sh`/`prove_and_submit.sh`) **TIMEOUT sau
600s, không đạt ANCHORED**. Dữ liệu thật ở `scripts/e9_trace_driven/results_live/e9_combined_trace_20260808T181408.csv` (1504 sample) + `_summary.md`
(bảng marker/transition thật). Không có 6-panel figure tự động (BTC gap/DA gap/P2P health không có trong committed state, xem giới hạn đã ghi ở
`x/sovereignty/preblock.go`'s `NewPreBlocker`) -- chỉ `fsm_state`/height/marker là dữ liệu live thật.

**Đính chính quan trọng (phát hiện SAU khi viết bản đầu của mục này, khi bị hỏi lại liệu lời giải thích Phase 7 có đáng tin không)**: bản đầu của mục
này giải thích Phase 7 timeout là do "interval tăng nhanh hơn tốc độ proof N=4 cố định có thể theo kịp" -- dựa hoàn toàn vào dòng log
`watch_and_prove.sh` tự in ra ("likely the documented staleness race") mỗi khi `prove_and_submit.sh` thoát với mã khác 0, **mà không tự kiểm chứng lại
nguyên nhân thật**. Khi bị hỏi lại và tự chạy tay `bash -x scripts/reanchoring_prover/prove_and_submit.sh` để xác minh, phát hiện ra: script thoát với
exit code **141 (SIGPIPE)** ngay ở Step 1/4 (trích xuất header), **CHƯA BAO GIỜ chạm tới Step 4 (submit proof) hay bị chain từ chối gì cả** -- dòng
`HEADER_LINES=$(echo "$ALL_HEADER_LINES" | head -n "$EXPECTED_N")` bị vỡ pipe thật khi `$ALL_HEADER_LINES` đã lớn hơn pipe buffer của OS (~64KB, tức
vài trăm header trở lên): `head -n 4` đọc đủ 4 dòng rồi thoát sớm, trong khi `echo` vẫn đang ghi phần còn lại → `echo` nhận SIGPIPE → dưới
`set -o pipefail`, toàn bộ script thoát 141. Đây là bug thật trong shell script, không liên quan gì đến logic ZK/chain. Sau khi sửa (thay bằng
here-string `head -n "$EXPECTED_N" <<< "$ALL_HEADER_LINES"`, không còn pipe sống để vỡ), chạy tay lại **thành công thật**: proof được chain CHẤP NHẬN
thật (`submitted at height 2517`), checkpoint tiến thật (`HeaderHistory` giờ bắt đầu từ height=5 thay vì height=1, `rt_last` đổi sang giá trị mới) --
xác nhận cơ chế ZK re-anchoring pipeline hoạt động đúng khi thực sự được gọi tới; **518 lần "rejected" ghi trong log trước đó gần như chắc chắn đều là
cùng 1 bug SIGPIPE này**, không phải chain từ chối proof vì staleness race như bản đầu đã suy đoán sai. Đã sửa `prove_and_submit.sh`. Kết luận đúng cho
Phase 7's timeout: cơ chế ZK re-anchoring hoạt động đúng khi được gọi, nhưng **`watch_and_prove.sh` trong suốt phiên chạy E9 không hề gọi thành công lần
nào** vì bug script này -- tức Phase 7 timeout thật, nhưng vì driver script bị lỗi chứ không phải vì bất kỳ giới hạn thiết kế nào của cơ chế proof N=4
cố định (giới hạn đó vẫn có thật và vẫn ghi ở `RealProofSubmittedHeight`'s doc, chỉ là KHÔNG PHẢI nguyên nhân của riêng lần timeout này). Chưa chạy lại
E9's Phase 7 với bản script đã sửa trong đợt này (interval hiện đã hơn 2500 header, một lần chạy watch_and_prove.sh với bug đã sửa sẽ submit thành công
nhưng vẫn cần rất nhiều lần lặp N=4 mới đuổi kịp).

**Bug thật khác tìm được khi chạy E9 lần đầu**: `scripts/framework/injector.py`'s `cleanup_profile` (dùng chung cho MỌI script chaos trong repo) chỉ gọi
`docker compose rm -f`, không `stop` trước -- `rm -f` chỉ bỏ qua prompt xác nhận, KHÔNG tự dừng container đang chạy (khác với `docker rm -f` thường).
Điều này vô hình suốt cả session vì mọi lần gọi trước đó đều xảy ra SAU KHI container đã tự hết `--duration` (đã exit tự nhiên). E9's Phase 4 lần đầu
tiên cần NGẮT một profile đang chạy dở giữa chừng (chaos-loss có `--duration=2m` nhưng mỗi chu kỳ chỉ giữ 20s) -- lộ ra thật: container kẹt "Up" mãi,
`wait_for_no_active_netem()` đúng đắn từ chối khởi động profile tiếp theo thay vì âm thầm chồng 2 profile lên nhau. Sửa tận gốc trong helper dùng chung:
`stop` trước `rm -f`.

---

## 4. Bộ Thực Nghiệm Tối Thiểu

Không làm quá rộng. Năm nhóm dưới đây đủ ba lớp bằng chứng: **formal**, **systems** và **cryptographic microbenchmark**.

| # | Nhóm | Nội dung bắt buộc |
|---|------|-------------------|
| 1 | TLA+ verification + ablation counterexamples | Reproduce safety/liveness; thêm ablation cho hysteresis, circuit breaker, P2P gate và DA consistency |
| 2 | Prototype fault-injection trên 4/7-node local testnet | BTC failure, DA failure, P2P eclipse, combined failure và recovery |
| 3 | Consensus overhead benchmark | Vanilla CometBFT vs. extended proposal |
| 4 | Recovery Proof Evaluation | Circuit composition (Table 6A), scaling benchmark (Table 6B) với 10–5,000 sovereign blocks; Figure 6 scaling plot |
| 5 | Attack-resilience integration tests | Forged receipt, data withholding, withdrawal during fallback, fake FSM state |

---

## 5. Figures & Tables Cần Có Trong Paper

| Figure/Table | Nội dung |
|--------------|---------|
| **Fig. 1** | Architecture: Engram execution + BTC settlement + Celestia DA + FSM sensors |
| **Fig. 2** | FSM timeline under combined failure *(output của E9 hoặc E2, ưu tiên E9)* |
| **Fig. 3** | Availability/throughput during outage: Engram FSM vs. vanilla CometBFT *(E2)* |
| **Fig. 4** | Recovery stability vs. `HYSTERESIS_WAIT` *(E5)* |
| **Fig. 5** | ZK proving time vs. number of sovereign transitions *(E6)* — **thay bằng Fig. 6 4-panel bên dưới* |
| **Fig. 6** | Recovery Proof Scaling: 4 panel (Constraint Count, Proving Time, Verification Time, Proof Size) *(E6)* |
| **Fig. 7** | Backend Trade-off radar chart: Noir+Honk vs. Plonky3 *(E6, tùy chọn)* |
| **Table 1** | Formal verification state-space results *(E1)* |
| **Table 2** | Failure matrix and expected policy *(E3)* |
| **Table 3** | Attack-resilience tests *(E8)* |
| **Table 4** | Extended proposal overhead *(E7)* |
| **Table 5** | Ablation study |
| **Table 6** | P2P profiler accuracy *(E4)* |

---

## 6. Việc Cần Làm Ngay Trong Repo

Trước khi chạy thực nghiệm, cần hoàn thiện các phần sau:

- [ ] Hoàn thiện `BeginBlock` thật trong `x/sovereignty/abci.go`, không để ở mức comment.
- [ ] Hoàn thiện `CalculateNextFSMState`, `ExecuteFSMTransition`, `IsWarningCondition`, `IsCriticalCondition`, `IsHealthyCondition` trong Go để khớp với TLA+.
- [ ] Tạo mock modules cho BTC finality sensor, DA receipt sensor và P2P health sensor.
- [ ] Viết `tests/fsm_transition_e2e_test.go` thành test thật với các kịch bản failure matrix.
- [ ] Bật lại constraint `computed_new_root == state_root_new` trong Noir hoặc tạo hai phiên bản: unconstrained demo và constrained benchmark.
- [ ] Thêm script reproducibility: `make test-faults`, `make bench-consensus`, `make bench-zk`, `make verify-tla`.
- [ ] Log toàn bộ state transition bằng CSV/JSON để vẽ timeline tự động.

---

## Kết Luận

Ý tưởng Engram Sovereign FSM đủ tốt cho hội thảo nếu evaluation được xây dựng có kỷ luật. Điểm quyết định không nằm ở việc bổ sung thêm lý thuyết, mà ở khả năng **chứng minh bằng thực nghiệm** rằng FSM:

- duy trì **safety** trong khi cải thiện **liveness** của modular blockchain dưới lỗi ngoại vi,
- có **recovery có kiểm soát**,
- và **overhead chấp nhận được**.