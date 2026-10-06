# Kế hoạch công việc: paper và source code

Mục tiêu: paper `docs/paper/` sẵn sàng nộp, tức là build được với `\draftfalse` (không còn
`\pending`) và mọi claim khớp với bằng chứng. Bằng chứng còn thiếu theo từng claim nằm ở
`docs/paper/STATUS.md`.

Nguồn yêu cầu, ghi ở từng mục:

| Ký hiệu | Nguồn |
|---|---|
| **[T]** | Feedback của thầy (định vị đóng góp, threat model, baseline, refinement, thu gọn thí nghiệm) |
| **[M]** | Review phương pháp luận (cách đo, cách báo cáo, tái lập) |
| **[B]** | Review blind "Major Revision" (research gap, liveness của cái gì, ZK) |

Trạng thái: ✅ xong · 🟡 còn dở (có mục "Cần làm") · ⬜ chưa làm.

Quy tắc chung:
- Mọi thay đổi giao thức đi theo thứ tự spec TLA+ → Go → TLC/Apalache → unit test → chạy live
  (skill `go-spec-fidelity`).
- Mọi đoạn viết mới trong paper theo skill `paper-writing`.
- Thí nghiệm live chạy **một lượt** trên code cuối (Giai đoạn 4), sau khi mọi thay đổi giao thức xong.

## Bảng tổng quan

| # | Việc | Trạng thái | Cần làm tiếp |
|---|---|---|---|
| 1 | Chốt các quyết định | 🟡 | 1g, Testbed B (xem mục 1) |
| 2 | Đặt trước tiêu chí thành công cho từng RQ | ⬜ | Viết vào `docs/EXPERIMENT.md` **trước** khi chạy lại |
| 3 | Điều tra bất thường reorg | ✅ | — |
| 4 | Phân tích lại dữ liệu | ⬜ | Làm trên dữ liệu của lượt chạy cuối |
| 5 | Security semantics theo mode + định lý withdrawal | ✅ | — |
| 6 | Lập luận lõi: vì sao consensus, liveness của cái gì, ZK phục vụ ai | ✅ | — |
| 7 | Reproducibility + định nghĩa chỉ số | 🟡 | Hash commit kết quả, URL/license, SD detection delay |
| 8 | Hệ nhãn bằng chứng + gọn phần giới hạn | ✅ | — |
| 9 | Sửa `spec/README.md` còn lại | ✅ | — |
| 10 | Smoke test live bản sửa sub-quorum certificate | ⬜ | Gộp vào lượt E7 đầu tiên của Giai đoạn 4 |
| 11 | Công cụ đo RQ4 | 🟡 | Đo xong ở tải thấp (n=5) và sweep 20→500 tx/s; chưa có trần; nguyên nhân FSM rời ANCHORED = timeout RPC DA (mục 30) |
| 12 | Baseline hard-gated (+ soft) | ✅ | E11 xong 27 trial (3 hệ × 3 lỗi × 3), đã vào paper (RQ2) |
| 13 | Bộ tấn công cộng tác f=2 (B1–B9) + replay checker | 🟡 | b8, b9 (40/40) xong; suite chính đang chạy lại từ ANCHORED; viết RQ1 một lần khi đủ (mục 13) |
| 14 | Làm lại A1–A10 ở n=7 trong phiên này | 🟡 | A3/A4 nằm trong suite f=2 chạy lại; A1, A2, A5–A10 đã xếp lịch (3 trial) |
| 15 | State root trong checkpoint Bitcoin | 🟡 | Chạy lại E10 trên code cuối |
| 16 | Hysteresis thích nghi ANCHORED↔SUSPICIOUS | 🟡 | Sweep tĩnh live xong (adaptive không hơn k=2); mô phỏng xong; live 3 pha và kẻ tấn công lặp lại đã xếp (mục 28) |
| 17 | Dữ liệu DA thật + backfill khoảng Sovereign | 🟡 | Live + control "inflated" xong; còn ablation Apalache và TLC lớp Server |
| 18 | Mô hình TLA+ của health certificate | ✅ | — |
| 19 | Refinement C1 | 🟡 | Giữ framing "layered + bounded"; tuỳ chọn chạy tiếp phần consensus-core |
| 20 | Chạy lặp các thí nghiệm live | ⬜ | Giai đoạn 4 |
| 21 | Chạy đa máy có WAN impairment | 🟡 | Hạ tầng sẵn; chưa có dữ liệu |
| 22 | Demo verifier ZK bên ngoài (tuỳ chọn) | ⬜ | Chỉ nếu còn thời gian |
| 23 | Tổ chức Evaluation | ✅ | Giữ RQ1–RQ4 |
| 24 | Hoàn thiện trước khi nộp | 🟡 | Xem mục 24 |
| 25–27 | Việc chờ quyết định và tồn đọng | — | — |
| 28 | Đo thêm hysteresis thích nghi (mô phỏng, live 3 pha, kẻ tấn công lặp lại) | 🟡 | Mô phỏng + đối chiếu keeper xong; live đã xếp lịch |
| 29 | Che sự cố DA khi vượt f theo ngưỡng k và vị trí Byzantine | 🟡 | Mô phỏng xong; live k=4 (h4a, h4i) đã xếp |
| 30 | Vì sao FSM rời ANCHORED dưới tải | 🟡 | Timeout RPC DA (xác nhận); probe CPU Celestia đã xếp; quyết định nới timeout |
| 31 | RQ3: lặp composite ≥ 5 trial và proof-only recovery với n lớn hơn | ⬜ | Chưa xếp lịch (khoảng 3–4 giờ) |
| 32 | Trình bày lại RQ4 (thuật ngữ n/arm/trial, hiệu ứng thứ tự, nguyên nhân DA) | ⬜ | Chờ quyết định hướng A/B/C |

Đã chuyển sang cấu trúc 9 section; paper dùng macro số liệu trong `docs/paper/results/result_macros.tex`.


## Cập nhật 5/10/2026 (phiên chạy thí nghiệm liên tục)

### Đã có dữ liệu hợp lệ trong phiên này
| Thí nghiệm | Kết quả chính | Nơi lưu |
|---|---|---|
| E11 (RQ2), 27 trial | Engram 96–102% tốc độ block, hard-gated 56/16/17% rồi không resume trong 120 s (9/9), soft lọt 31–72 withdrawal sau thời điểm khóa | `e11_hard_gated_baseline/results_live/` |
| E7 throughput, 3 nhánh × 5 mức × 5 lần | Không nhánh nào bão hòa đến 500 tx/s; p95 Engram tăng từ 100 tx/s, vanilla phẳng; chưa có trần | `e7_consensus_overhead/results_live/throughput_*` |
| Chẩn đoán tải 200 tx/s (3 lượt) | FSM rời ANCHORED vì cờ lỗi DA: `blob.Submit` quá 20 s, `blob.GetAll` quá 800 ms; không phải độ trễ P2P | `diag_load_demotion_*.json` |
| E5 hysteresis live, 33 ô + `StableWindow=64` | adaptive 8,3 flap so với k=2 là 9,0; `T=64` là 3,0; static k≥4 là 0 | `e5_hysteresis_flapping/results_live/` |
| Mô phỏng hysteresis (keeper thật, 200 seed ghép cặp) | adaptive giữ phát hiện 2 block lúc yên, chống burst ngắn lặp lại (1 so với 300 hạ cấp), không thắng static k=4 về flap | `tests/e2e/results/e5c_*.csv` |
| Che sự cố DA theo vị trí Byzantine (mô phỏng) | b≤2: luôn phát hiện đúng k block; b=3: k=2 phát hiện cả 35 vị trí, k=3 bỏ sót 14, k=4 bỏ sót 28 | `tests/e2e/results/e8_hide_outage_placement*.csv` |
| E8 b9, 5 trial × 8 lần, mỗi lần từ ANCHORED | b≤2: 0 block ngoài khoảng trung thực, SOVEREIGN không bị ép; b=3: 40–42 (force) và 5–12 (hide) block vi phạm, SOVEREIGN bị ép 5/5, sự cố vẫn bị phát hiện 5/5 | `collusion_suite_20261005T100618.csv` |
| E8 b8 (1 trial), b5 (3 trial), c0 (đối chứng khởi động lại) | b8 pass; b5 105–110 lần từ chối `withdrawal_locked`; c0: node trung thực vẫn chuyển SUSPICIOUS không có tấn công | `collusion_suite_*` |

### Lỗi harness đã sửa (ảnh hưởng đến cách đọc dữ liệu cũ)
- Mọi lần chạy phải bắt đầu từ ANCHORED (trước đây b7a/b7b, b4, và b9 từ trial 4 bắt đầu ở SOVEREIGN); b9 dựng chuỗi mới mỗi trial.
- Tiêu chí dựa vào "có chuyển trạng thái" bị loại: nhiễu nền gây chuyển trạng thái cả khi không có tấn công (đối chứng c0). b9 giờ dùng Lemma 1 (số block ngoài khoảng trung thực) và "vào SOVEREIGN".
- CSV được ghi sau từng lần chạy; `docker logs` timeout nâng lên 900 s.
- Cột `byz_proposed_heights` từng bằng 0 vì log container Byzantine bị mất khi tạo lại container; nay đọc trước khi tạo lại.
- Dữ liệu cũ không hợp lệ được cách ly trong `_invalid_*` (không commit); mọi số cũ về f=2 phải được thay bằng lượt chạy lại.

### Lịch chạy tự động (một script điều khiển cụm tại một thời điểm, ước tính lúc 16:30 ngày 5/10)
| # | Nội dung | Dự kiến xong |
|---|---|---|
| 1 | Suite f=2 chính chạy lại từ ANCHORED (a3, a4, b1, b2, b3a, b3b, b4, b6, b7a, b7b × 3) | 18:00 |
| 2 | Tấn công đơn lẻ A1, A2, A5–A10 × 3 trial ở n=7, dữ liệu của phiên này (không dùng kết quả 2/9) | 20:30 |
| 3 | Probe CPU Celestia dưới tải 200 tx/s (cgroup throttling) | 21:10 |
| 4 | Live hide_outage k=4: Byzantine liền kề (h4a) và xen kẽ (h4i), 3 trial | 23:15 |
| 5 | Live 3 pha hysteresis (static k=2, k=4, adaptive `T`=16/64/128, adaptive 2→4 `T=64`), 2 độ trễ, 3 trial | 10:15 (6/10) |
| 6 | Live kẻ tấn công lặp lại (xung ngắt bridge DA), 5 cấu hình × 3 trial | 15:00 (6/10) |

### Chưa xếp lịch (cần quyết định)
- **RQ3 (yếu nhất):** lặp composite ≥ 5 trial và proof-only recovery với n lớn hơn. Chờ kết quả live hysteresis chỉ giải quyết nửa hysteresis của RQ3, không giải quyết hai chỗ này.
- Thử nới thời gian chờ DA (800 ms, 20 s) và đo lại throughput (khoảng 2 giờ), tùy kết quả probe CPU.
- Hướng sửa RQ4 (A: chuẩn hóa thuật ngữ, B: ghi hiệu ứng thứ tự, C: chạy lại mỗi mức tải một chuỗi mới).
- Monte Carlo có SD của độ trễ phát hiện (sẽ ghi đè CSV E4 đang sửa dở; cần ghi vào thư mục riêng).
- Testbed B (đa máy): để sau, bạn tự chạy; tôi chỉ chuẩn bị mã.

### Đã sửa trong paper (không cần chạy)
- RQ2 chỉ hỏi về Bitcoin và DA, kèm chi phí (withdrawal lọt trước khi khóa); P2P chuyển sang RQ1.
- Abstract: baseline là cài đặt của nhóm, n=3 một máy, chi phí phát hiện 18–56 withdrawal, con số 0 sau khóa là kiểm tra nhất quán.
- Hysteresis hạ xuống thành thành phần phân tích: bảng mô phỏng, phát biểu đánh đổi nhiễu so với che giấu, câu hạn chế cũ được sửa.
- Mâu thuẫn A6 n=4 trong Discussion được sửa thành A1/A2 n=4.
- Còn `\pending`: B1–B6 (RQ1), composite ≥ 5 trial, cột live của bảng hysteresis, kẻ tấn công lặp lại live, hide_outage k=4 live, A1/A2 n=7, SD detection delay, hash commit, tag phát hành.

---

## Giai đoạn 4 — Chạy lại toàn bộ thí nghiệm live trên code cuối

### Vì sao phải chạy lại
Mọi số liệu live hiện có được đo trước khi có kiểm tra round binding trong `ProcessProposal`
(`3e48979`), và nhiều số liệu được đo trước hysteresis thích nghi, backfill gate hoặc state root
trong checkpoint. Mỗi số trong paper phải ghi được commit đã sinh ra nó.

### Điều kiện trước khi chạy
1. Chốt code: mọi thay đổi giao thức còn dở (mục 16, 17) xong; working tree sạch; ghi commit hash.
2. Mục 2 xong và đã commit (tiêu chí đặt trước khi có số mới).
3. `go build ./...`, `go vet ./...`, `go test ./...` (chưa chạy toàn bộ), `golangci-lint run`.
4. Chạy lại phần formal (mục 17, 19) **tách riêng** với các lượt live; CPU của TLC làm sai số đo thời gian.
5. Triển khai sạch: xoá `testnet-data/`, `data/bitcoin-node0*`, `data/celestia-*` cùng lúc, rồi
   `make testnet-up N=7 PROVER=host`.
6. Đặt `ENGRAM_PARAM_PROOF_ONLY_RECOVERY=true` trong `.env` trước `make testnet-up` (heuristic
   tắt; mặc định trong code là `false`). Ghi giá trị này vào summary của mỗi lượt để biết lượt nào
   dùng proof-only.
7. Sau mỗi thí nghiệm: skill `experiment-results-refresh` (xoá dữ liệu bị thay thế, dựng lại hình,
   cập nhật `docs/EXPERIMENT.md`), commit dữ liệu riêng, rồi cập nhật macro và `\pending` trong paper.

### Thứ tự chạy và dữ liệu cần làm lại

Không chạy song song hai thí nghiệm. Thí nghiệm không lỗi đi trước để xác nhận kiểm tra mới không
từ chối proposal trung thực.

| Thứ tự | Thí nghiệm | Dữ liệu hiện có | Cần chạy lại / chạy mới |
|---|---|---|---|
| 1 | **E7** overhead (`e7_consensus_overhead/`) | Bốn nhánh: 1 process, 7 vanilla không ext, 7 vanilla ext, Engram; trước round binding | Chạy lại nhánh Engram; xác nhận 0 REJECT `certificate_not_bound_to_commit`, 0 nil prevote, có ca round > 0 (mục 10); thêm trần throughput 100→200→500→1000 tx/s (chưa có) |
| 2 | **E11** baseline (`e11_hard_gated_baseline/run_matrix.sh`) | ✅ 27 trial (5/10); hình height theo thời gian đã sinh, chưa đưa vào paper | Không cần chạy lại; thêm trial chỉ nếu muốn n=5 |
| 3 | **E2** S1–S7 (`confidence_interval_recovery_time.py`) | Một lượt cũ | S1–S7 × 5, đảo thứ tự S2–S6, thêm S5 độc lập; khoảng 2–2,5 giờ |
| 4 | **E9** composite (`e9_composite_fault/`) | Một lượt | Composite × 5, P4 × 3 |
| 5 | **E3** (`e3_failure_matrix/`) | Backfill 120/300/600 s, negative control (4/7 giữ, 3/7 mở) | Chạy lại trên code cuối; thêm control "hai validator báo contiguous height phóng đại" (chưa có dữ liệu) |
| 6 | **E5** (`e5_hysteresis_flapping/`) | ✅ Sweep tĩnh + thích nghi (33 ô) + `StableWindow=64` (6 ô) | Live 3 pha và kẻ tấn công lặp lại đã xếp (mục 28) |
| 7 | **E8** (`e8_attack_resilience/live_collusion_suite.py`) | b8, b9 (40/40), b5, c0 hợp lệ; suite chính đang chạy lại từ ANCHORED | Hết suite chính; A1, A2, A5–A10 × 3 ở n=7 đã xếp (dữ liệu của phiên này); A11 round-replay live chưa làm |
| 8 | **E4** (`e4_p2p_eclipse_detection/`) | Churn, relay latency, Sybil (cũ) | Chạy lại |
| 9 | **E10** (`e10_bitcoin_reorg/`) | Sweep reorg cũ | Depth 6–14, mỗi depth ≥ 3 lần; kiểm checkpoint 44 byte |
| 10 | **E6** (`e6_zk_reanchoring_benchmark/`) | Benchmark ZK | Chỉ khi đổi circuit hoặc toolchain; cần máy rảnh |
| 11 | **Testbed B** (mục 21) | Không có | Nếu quyết định làm |

E1 (formal) chạy tách riêng, trước hoặc sau toàn bộ phần live:
- Lớp Server với backfill và hysteresis thích nghi (chưa chạy lại).
- Ablation bỏ điều kiện backfill (dòng mới cho `tab:ablation`).
- Ablation fast-forward liveness (chưa hoàn tất; lần chạy tốt nhất tới depth 12, không vi phạm).
- `MC_ConsensusCoreRefinementSafety` nếu muốn tiếp tục mục 19.

### Sau khi chạy xong
Cập nhật `docs/paper/results/result_macros.tex`, các bảng, và thay các `\pending`: baseline hard-gated,
B1–B6 ở n=7, sweep hysteresis, ghi nhận lượt nào dùng proof-only, negative control backfill, hash commit kết quả.
Cập nhật bảng claim–evidence trong `docs/paper/STATUS.md`.

---

## Việc còn dở theo mục

### 1. Chốt các quyết định (🟡)

| # | Câu hỏi | Trạng thái |
|---|---|---|
| 1b | Hạ tầng đa máy | ✅ Cloud nhiều vùng: `deploy/` (Ansible + Terraform AWS/GCP/DigitalOcean) |
| 1c | Baseline soft/optimistic | ✅ Có (chế độ `soft`) |
| 1d | Hard-gated khi BTC tạm dừng: giới hạn `round` | ✅ `min(round, 3)` (`x/sovereignty/types/baseline.go`) |
| 1e | Mức tham vọng formal | ✅ Thử hai bước; kết quả ở mục 19, framing "layered + bounded" |
| 1f | Thứ tự 5 contribution | ✅ Certificate và semantics theo mode đứng đầu |
| 1g | Phương án dự phòng RQ1/RQ2 nếu thiếu số liệu | Chưa chốt mốc ngày |
| mới | Có làm Testbed B | Chờ quyết định (mục 21) |

Phương án dự phòng (1g): RQ2 viết thành "degrade an toàn" (chain tiếp tục, withdrawal khoá đúng
lúc, không lệch AppHash, replay 0 vi phạm), không claim "cải thiện liveness" so với baseline; RQ1
thu hẹp về phạm vi đã test, chuyển collusion sang limitations.

### 2. Đặt trước tiêu chí thành công cho từng RQ (⬜) [M]

Chưa có trong `docs/EXPERIMENT.md`. Làm **trước** lượt chạy cuối và commit riêng để có mốc thời gian
trong git; paper dùng đúng các ngưỡng này, và mọi từ "correctly/bounded/acceptable/cadence held" phải
gắn với một ngưỡng hoặc bị bỏ. Mỗi RQ gồm: metric, ngưỡng đạt, oracle, số lần lặp. Dự thảo (cần chốt):

| RQ | Metric | Tiêu chí đạt |
|---|---|---|
| RQ1 | Vi phạm replay mỗi case B1–B8 | 0 vi phạm trên mọi trial |
| RQ1 | Sweep B9, b ≤ 2 | 0 height có aggregate ngoài khoảng honest; 0 withdrawal không an toàn |
| RQ1 | Sweep B9, b = 3 (f+1) | Đối chứng âm: vi phạm xuất hiện, và chỉ ở height có proposer Byzantine |
| RQ1 | AppHash divergence | 0 height lệch |
| RQ2 | Block trong cửa sổ lỗi, Engram vs. hard-gated | Engram ≥ 80% số block lúc khoẻ cùng thời lượng; hard-gated = 0 khi DA outage |
| RQ2 | "Không stall" | Không có khoảng giữa hai block > 3 × block interval lúc khoẻ |
| RQ3 | Số flap mỗi giờ khi không có lỗi | ≤ 1 flap/giờ ở WAN profile chuẩn |
| RQ3 | Độ trễ phục hồi sau khi lỗi hết | ≤ hysteresis wait + thời gian prove (khai báo con số) |
| RQ4 | Giảm block rate so với vanilla | Báo CI 95% của hiệu số; ngưỡng "acceptable" khai báo trước |
| Attack | "Cadence held" | Block rate giảm < 10% so với trước tấn công |

Oracle cho S1–S7: bảng trạng thái FSM kỳ vọng cho từng kịch bản (ví dụ S5 chỉ SUSPICIOUS thoáng
qua, không SOVEREIGN), viết trước khi chạy lại.

### 4. Phân tích lại dữ liệu (⬜) [M]
Làm trên dữ liệu của lượt chạy cuối, dùng định nghĩa chỉ số ở mục 7 và tiêu chí ở mục 2:
- Đếm flap trực tiếp từ log transition của S1–S7 và composite/P4; bảng time-to-detect và
  time-to-recover cho từng kịch bản.
- Hysteresis in-process: nhiều seed (ví dụ 10) để có phương sai; ghi mẫu số của absorption rate.
- Cột độ trễ phục hồi theo ngưỡng trong bảng hysteresis; đưa số live spot-check vào bảng, không chỉ vào hình.

### 7. Reproducibility (🟡)
Phần lớn đã có (môi trường, phiên bản, tham số netem, định nghĩa chỉ số). Còn lại: hash commit sinh
kết quả, repository URL và license, SD của detection delay, RTT liên vùng nếu có Testbed B. Danh sách
`\pending` hiện tại nằm ở `docs/paper/STATUS.md`.

### 10. Smoke test live bản sửa sub-quorum certificate (⬜)
Unit test chỉ phủ `ProcessProposal`; luồng PrepareProposal → CometBFT → ProcessProposal chưa được chạy
live trên code cuối. Gộp vào lượt E7 đầu tiên của Giai đoạn 4. Đạt khi `grep -c
'reason=sub_quorum_certificate'` và `grep -c 'reason=certificate_not_bound_to_commit'` đều bằng 0 trên
mọi node chạy khoẻ 30 phút, block rate như chế độ extended của E7, và S3 (DA outage) vẫn đi vào nhánh
degraded. Cần thêm một ca có round > 0 ở height trước.

### 11. Công cụ đo RQ4 (🟡)
Công cụ (`consensus_metrics.py`, `live_overhead_scan.py`, `live_rq4_metrics.py`, nhánh 7 vanilla) đã có.
Còn lại: trần committed throughput (20 → 100 → 200 → 500 → 1000 tx/s); chạy lại E7 trên code cuối.
Microbenchmark 18,25 µs: báo số lần lặp và phương sai (`-count=10`, `benchstat`), nói rõ không gồm
verify proof (đo riêng).

### 12. Baseline hard-gated (✅) [T][B]
Chế độ đã có: `engramd start --hard-gated|--soft` hoặc `ENGRAM_MODE`, `make testnet-up MODE=…`,
`scripts/e11_hard_gated_baseline/run_matrix.sh`. Dữ liệu: 27 trial xong ngày 5/10 (đã vào RQ2). Thiết kế đã chạy: mỗi trial 60 s khoẻ → 180 s
lỗi → 120 s phục hồi; lỗi = BTC tạm dừng, DA outage (cả 7 bridge), cả hai; 3 trial × 3 lỗi × 3 hệ, genesis
mới cho mỗi hệ. Metric: block commit trong cửa sổ lỗi, khoảng lặng dài nhất, số round ở height bị kẹt,
thời gian ra block đầu tiên sau phục hồi, số withdrawal không an toàn. Kết quả điền vào bảng RQ2 và
hình height theo thời gian.

### 13. Bộ tấn công cộng tác f=2 (🟡) [T][M][B]
Code (hook Byzantine, replay checker `engramd debug replay-fsm`, `live_collusion_suite.py`, test
in-process B1–B3, B5, B6, B9) đã có. Dữ liệu live trước đây (một trial B1–B5 trước round binding) không còn dùng; dữ liệu cũ của phiên này bị loại vì bắt đầu sai trạng thái. Đã xong: b5 (3 trial), b8 (1), b9 (5 trial × 8 lần), c0. Suite chính đang chạy lại từ ANCHORED. Cần đủ:
- B1–B7 × 3 trial, B8 × 1 trial (≥ 10 phút), B9 sweep b ∈ {0, 1, 2, 3} × 2 mục tiêu × 5 trial.
- A1/A2 (eclipse, Sybil) ở n=7; A11 round-replay ở live.
- Hình B9: trục x = b, trục y = tỷ lệ height do Byzantine kiểm soát, tô vùng b ≤ f.
- Replay checker cho mỗi case; paper báo số dòng vi phạm (kỳ vọng 0; b = 3 là đối chứng âm).
- B7/B8 chỉ đo, không khẳng định chuyển trạng thái; n=1 (B8) và n=3/5 chỉ đủ báo số lần đạt.

### 14. Làm lại A1–A10 ở n=7 trong phiên này (🟡) [M]
A3, A4 nằm trong suite f=2 chạy lại; A1, A2, A5–A10 đã xếp lịch (3 trial), dùng dữ liệu của phiên này, không dùng lần chạy 2/9. Ghi chú: A3 (khai attestation DA +1) **được chấp nhận** vì nằm trong dung sai của `da.VerifyReceipt`, nên kỳ vọng "honest từ chối" của suite là sai; nêu như giới hạn DA-trust. Các yêu cầu gốc:
Các dòng này chưa chứng minh được cơ chế phòng thủ đã kích hoạt:
- A3 (data withholding), A4 (forged BTC receipt): đếm `ProcessProposal REJECT reason=…` theo lý do
  trên các validator trung thực.
- A7 (censorship): cho nhiều proposer cùng kiểm duyệt (node04 + node05) hoặc giảm `MaxIgnoreRounds`
  để `IsCensoring` thực sự bắn.
- A10 (timeout flood): quét 20, 50, 100, 200, 500 msg/s đến khi limiter bắt đầu drop; báo block rate
  theo cường độ so với ngưỡng "cadence held" (mục 2).

### 15. State root trong checkpoint Bitcoin (🟡) [T][B]
Đã làm: payload 44 byte (`ENGR` ‖ height ‖ block hash CometBFT), `VerifyAnchor` so root với block hash
cục bộ, unit test. Paper nói rõ accumulator Poseidon của proof không được ghi vào Bitcoin. Còn lại:
chạy lại E10 trên code cuối để kiểm tra hồi quy.

### 16. Hysteresis thích nghi (🟡) [T]
Kết quả phiên 5/10: sweep live tĩnh + thích nghi xong (adaptive không hơn static k=2; `T=64` giảm còn 3,0 flap). Mô phỏng và các thí nghiệm đo thêm ở mục 28.
Đã làm: spec (`flap_level`), Go (`MaxDownHysteresisThreshold`, `StableWindow`, `NextFlapLevel`), unit
test, mô tả trong paper. Còn lại:
- Sweep live cạnh xuống (E5): k=2 tĩnh, k=6 tĩnh, thích nghi (k0=2, k_max=8, W=100), 3 trial mỗi cấu
  hình, hai chế độ: jitter WAN không lỗi (số flap, uptime) và WAN + S2 BTC congestion (độ trễ phát hiện
  và phục hồi). Kỳ vọng: thích nghi bằng k=6 về flap và bằng k=2 về độ trễ phát hiện.
- Xác nhận `MC_FSMSafety`/`MC_FSMLiveness` và các ablation Apalache đã chạy trên spec cuối.

### 17. Backfill khoảng Sovereign (🟡) [T][B]
Đã làm: publisher dữ liệu block thật, `da_contiguous_height`, `RecoveringEntryHeight`, điều kiện trong
`CalculateNextState`, spec (`da_backfill_valid`, `SovereignDataPublishedSafety`, `MC_FSMSafety` và
`MC_FSMLiveness` 0 vi phạm), live backfill 120/300/600 s với `verify_backfill.py` (khớp 100%),
negative control 4/7 giữ và 3/7 mở. Còn lại:
- Ablation bỏ điều kiện backfill (Apalache) và dòng mới trong `tab:ablation`.
- Chạy lại TLC lớp Server (`ServerDABackfill`).
- Control "hai validator ký contiguous height phóng đại không mở được cổng": đã có (2 phóng đại + 5 giữ, 2/2 giữ).
- Viết kết quả negative control vào `evaluation` và bỏ `\pending` tương ứng.
- Chạy lại toàn bộ trên code cuối (Giai đoạn 4).

### 19. Refinement C1 (🟡) [T]
Đã làm: tách làm hai phần. Phần môi trường hoàn tất (17.500 trạng thái phân biệt, 4.826.336 sinh ra, độ sâu 10,
10 phút 35 giây, không vi phạm). Phần consensus-core chưa hội tụ (dừng sau 1 giờ 41 phút ở 19.848.141
trạng thái phân biệt, độ sâu 15, không vi phạm; độ sâu 12 là của ablation fast-forward). Paper giữ framing "layered
specification + bounded model checking" và nêu refinement còn mở.

Định hướng formal sau khi xong live Docker (không chạy TLC/Apalache khi cụm đang chạy):
1. Ablation backfill: chạy `MC_Ablation_NoDABackfillSafety_FSMOnly_Apalache` (đã viết, chưa chạy); có vi phạm thì thêm
   hàng vào `tab:ablation`. Thay ablation f+1 (chưa có kết quả đáng tin) bằng ablation này.
2. Chạy lại ablation hysteresis và P2P gate trên FSM hiện tại: hai module `EngramFSM_Ablation_*` là bản sao đã lệch
   khỏi `EngramFSM.tla`.
3. C1 consensus-core: chạy cấu hình nhỏ hơn (N=4, f=1, 2 vòng, độ cao tối đa 2, MaxTimestamp=6 là cấu hình hiện tại);
   chỉ dùng nếu TLC xác nhận vẫn đạt tới đổi vòng, proposer Byzantine, chuyển trạng thái FSM và block được quyết định.
4. C1 bước xen kẽ: lập luận tay rằng bước môi trường không đổi biến của ánh xạ refinement.
5. Kiểm tra quy nạp Apalache cho C1: file khung `EngramServerRefinement_Ind.tla` chưa tồn tại, phải viết mới;
   giới hạn một ngày.
6. Cập nhật Section 5, `spec/README.md` và báo cáo (mục 6, II.A) bằng số mới.
Điều kiện dừng: nếu C1 vẫn mở, giữ framing hiện tại.

### 28–32. Việc mới của phiên 5/10
Xem bảng "Cập nhật 5/10/2026" ở trên. Tóm tắt: (28) đo thêm hysteresis, (29) che sự cố theo ngưỡng k và vị trí, (30) nguyên nhân FSM rời ANCHORED dưới tải là timeout RPC DA, (31) RQ3 cần composite ≥ 5 và proof-only với n lớn hơn (chưa xếp), (32) trình bày lại RQ4.

### 33–35. Ba phản biện giả định của reviewer (đợt 6/10)
Ba nhận xét mà một reviewer hệ thống nhiều khả năng đưa ra; tất cả đều đúng ở phần lập luận. Thứ tự ưu tiên: sửa văn bản
(vài giờ, không chạy) trước, rồi mô phỏng, rồi thí nghiệm.

**33. "Đánh giá vẫn là một máy" (Major 1) (🟡) [T][M]**
Toàn bộ validator, Bitcoin, Celestia nằm trên một máy; validator không có giới hạn CPU/RAM riêng, nên nhiễu lập lịch có thể bị
cảm biến hiểu thành suy giảm mạng (Engram lấy độ trễ, chất lượng peer, churn làm đầu vào FSM). Đây cũng là nút thắt chưa tách
được ở RQ4 (FSM rời ANCHORED khi tải tăng).
- Việc: Testbed B trên DigitalOcean, 7 validator ở 7 trung tâm dữ liệu (nyc3, tor1, ams3, fra1, sfo3, sgp1, blr1) và `infra01`
  ở nyc3. Thiết kế hệ thống, checklist và kế hoạch chạy nằm trong `deploy/README.md`. Phạm vi: composite ×3 đến ×5, baseline
  RQ2 (E11) tuỳ chọn, quét tải tuỳ chọn (cần nối E7 vào `scripts/framework/cluster.py`, hiện chỉ E9 và E11 chạy qua backend SSH).
- Quét n = 4, 7, 13, 25, 50, 100 bằng micro-benchmark offline trên mã thật: kích thước chứng chỉ, thời gian gộp, thời gian xác
  thực proposal; chạy khi máy chính rảnh hoặc trên Air. Chưa viết.
- Sửa văn bản ngay: nêu rõ validator không có giới hạn tài nguyên riêng và kết luận RQ4 ở tải cao chịu ảnh hưởng của việc dồn một
  máy.
- Chưa chạy lần nào trên máy thật; chi phí khoảng $0,39 mỗi giờ.

**34. Baseline hard-gated trộn hai yếu tố (Major 2) (⬜) [T][B]**
Phát biểu "hard-gated không tự phục hồi, Engram thì có" gộp (a) mất liveness trong lúc lỗi, đúng cho mọi chuỗi hard-gated, và
(b) không tự chạy lại sau lỗi, chỉ đúng cho cài đặt này: checkpoint mà đề xuất cần chỉ được tạo cho block mới, mà chuỗi đang
không commit được block mới (vòng phụ thuộc, phải khởi động lại tay).
- Sửa văn bản ngay: ghi "trong cài đặt này", tách (a) và (b), không dùng baseline hiện tại làm bằng chứng về mọi chuỗi hard-gated.
- Thêm baseline thứ hai cùng binary: hard-gated với bộ theo dõi neo định kỳ nộp checkpoint của block đã commit mới nhất dù
  không có block mới (`x/sovereignty/proposal.go` ép trạng thái ANCHORED ở chế độ hard-gated; phần nộp checkpoint nằm ở `x/anchor`,
  chưa đọc để ước lượng). Chạy lại ba loại lỗi × 3 lần; đo thời gian tự resume sau lỗi.
- Diễn giải kết quả mới như một đánh đổi: hard-gated độc lập dừng trong lúc lỗi nhưng không lọt giao dịch rút; Engram giữ
  liveness nhưng lọt một số trước khi khoá. Có thể thu hẹp khoảng cách; nói đúng như vậy.
- Công sức ước lượng: nửa ngày sửa, 2–3 giờ chạy.

**35. Lỗi Bitcoin và con số phơi nhiễm (Major 3) (🟡) [T][M]**
Ngưỡng đặt theo số block Bitcoin (Suspicious 5, Sovereign 8, K=2) và regtest nhanh khoảng 30 lần mainnet (một block mỗi 20 giây).
Thời gian tới SOVEREIGN: 117 giây (RQ2), 141,6 ± 13,8 giây (S2) tương đương khoảng 58 và 71 phút trên mainnet. Con số "18–19 và
45–56 giao dịch rút lọt trước khi khoá" phụ thuộc tải (một giao dịch rút mỗi 2 giây) và Bitcoin tăng tốc, không phải mức phơi
nhiễm sản xuất. Kịch bản "congestion" chỉ là dừng nộp checkpoint, và "DA outage" là dừng bridge, không phải DAS hay withholding.
- Sửa văn bản ngay: báo T_exposure = t_Sovereign − t_fault theo số block Bitcoin, theo số block Engram, theo giây trên testbed và
  theo phút tương đương mainnet; ghi số giao dịch rút là phụ thuộc tải; thêm cận trên N ≤ (số block Engram trong cửa sổ) × (số giao
  dịch rút tối đa mỗi block, lấy từ thông lượng đo được).
- Mô phỏng tại chỗ (nửa ngày, không cần cụm): luật FSM thật với thời gian giữa các block Bitcoin kiểu Poisson 10 phút, đo (i) thời gian
  tới SOVEREIGN sau khi dừng nộp checkpoint, (ii) tỉ lệ báo động nhầm khi chạy bình thường ở ngưỡng 5/8, K=2.
- Thảo luận: giới hạn tốc độ rút tiền ở SUSPICIOUS sẽ chặn cận trên N nhưng chưa cài; ghi là hướng cải tiến.
- Tiến độ (6/10): đã viết vào paper (Discussion, bảng `tab:exposure` và `tab:exposure-design`; RQ2 và Methodology có câu qualify).
  B1 xong: `tests/e2e/mainnet_cadence_test.go` (FSM thật cộng mô hình chu kỳ checkpoint), kết quả `tests/e2e/results/e5e_*.csv`.
  Mô hình khớp testbed (thời gian tới SOVEREIGN 129 giây, p5-p95 91-149 giây, so với 117 và 141,6 đo được; quiet-phase ở k=2 0,80-0,84
  và k=4 1,00, so với 0,79-0,87 và 1,00 đo được). Ở nhịp mainnet: 49,6 phút (steady), 46,7 phút (Poisson, 18-79). Dự đoán mới: ở nhịp
  mainnet, chuỗi khoẻ leo thang lên SOVEREIGN khoảng một lần trong tám ngày (10 sự kiện trong 77 ngày), vì `MaxSuspiciousTime`=24 block
  Engram đặt cho nhịp 20 giây. B2 xong: `scripts/e5_hysteresis_flapping/baseline_gap_cycle.py` (`btc_gap` khoẻ chạm 5 ở 3-16% mẫu;
  quiet-phase k=2 0,82, k=4 1,00).
- B3 đã xếp lịch (`chain_b3`, sau seven-scenario): miner 600 giây, baseline 40 phút rồi dừng nộp checkpoint, 3 chuỗi mới, khoảng 5-6 giờ.
  Kiểm chứng phép quy đổi và dự đoán thời gian tới SOVEREIGN; không đủ dài để thấy leo thang giả (một lần mỗi tám ngày).
- B4 (hạn mức rút phân bậc, mặc định tắt, kèm đặc tả TLA+, test và một lần chạy bật/tắt) chưa làm. Cảnh báo: mỗi lần triển khai build image
  từ cây mã hiện tại, nên KHÔNG sửa `x/` khi hàng đợi thí nghiệm đang chạy; phát triển trong một `git worktree` riêng hoặc sau khi hàng đợi xong.
- Kiểm tra nên làm: đặt `MaxSuspiciousTime` theo nhịp Bitcoin và chạy lại mô phỏng để xem leo thang giả biến mất.

### 21. Chạy đa máy có WAN impairment (🟡) [T][M]
Kế hoạch chi tiết, thiết kế hệ thống và checklist: `deploy/README.md` (mục 33). Hạ tầng đã có: `deploy/` (Ansible + Terraform cho AWS, GCP, DigitalOcean) và backend SSH của
`scripts/framework`. Chưa có dữ liệu. Nếu làm: 7 VM validator ở ≥ 3 vùng + 1 VM hạ tầng, composite
fault × 3 trial, đặt cạnh số liệu single-host; ma trận WAN đầy đủ chỉ cho composite, lỗi đơn ở một mức
WAN. Khi có số liệu, bật `\testbedBtrue` và thêm cột Testbed B.

### 22. Demo verifier ZK bên ngoài (⬜, tuỳ chọn) [B]
Xuất verifier Solidity bằng `bb` (kiểm tra phiên bản `bb` có hỗ trợ UltraHonk), chạy trên EVM devnet,
verify recovery proof thật, đo gas. Nếu không làm, §4.7 giữ ở mức mô tả verifier mục tiêu.

### 24. Hoàn thiện trước khi nộp (🟡)
- Đối chiếu `references.bib` với DBLP cho các entry chưa chắc: `vesely2022plumo`, `azouvi2022pikachu`,
  `karakostas2021checkpointing`, `braithwaite2020tendermint`, `hayashibara2004phi`.
- Compile trên Overleaf (gói `forest`); kiểm tra cây taxonomy và các hình không tràn lề.
- URL artifact và license (`09_conclusion.tex`).
- Rà toàn bài theo checklist skill `paper-writing` mục X.
- `\draftfalse` build thành công, `grep -rn pending docs/paper/sections/` rỗng.

---

## Việc chờ quyết định và tồn đọng

### 25. Chờ quyết định: thí nghiệm Celestia light client (DAS) song song
Ở quy mô này, bridge cho mỗi validator (tải và kiểm tra toàn bộ block) mạnh hơn DAS sampling; light
client chỉ thêm tính thực tế. Cách ít rủi ro: container `celestia-light-node0X` chạy cạnh bridge như
một thí nghiệm riêng, `x/da/publisher.go` vẫn dùng bridge. Cần đồng ý trước khi làm.

### 26. Chờ quyết định: validator dùng Bitcoin light client (SPV thật)
`IsBtcSpvFailed` không phải SPV: `VerifyAnchor`/`BlockContainsTag` gọi `getblock` verbosity=2 lên
`bitcoind` full node của từng validator. Việc nên làm là đổi tên (ví dụ `IsAnchorReverificationFailed`);
paper đã dùng "anchor re-verification". SPV thật (BIP157/158, ví dụ `btcsuite/neutrino`) chỉ đáng làm
nếu validator bỏ full node. Thay đổi kiến trúc; cần đồng ý trước khi làm.

### 27. Tồn đọng
- `reanchoring-prover` chạy trong container bị OOM quá 8192M kể cả với Docker Desktop VM 12 GB; dùng
  `watch_and_prove.sh` trên host (`make testnet-up PROVER=host`). Chi tiết trong comment của
  `docker/reanchoring-prover.yml`.
- `docs/FEEDBACK.md` giữ untracked/riêng tư, không commit.

---

## Đã xong (chỉ còn tiêu đề)

- ✅ 3. Điều tra bất thường reorg — phụ lục reorg giải thích đường đi qua gap, không qua cờ critical.
- ✅ 5. Security semantics theo mode + định lý withdrawal — §3.2 và Theorem 1 ở §5.1.
- ✅ 6. Lập luận lõi — vì sao quyết định nằm trong consensus (Introduction), khung ba trục trong §8.1,
  verifier mục tiêu của proof (§4.7).
- ✅ 8. Hệ nhãn bằng chứng (Lemma, Theorem, Property, Observed) + gọn phần giới hạn.
- ✅ 9. `spec/README.md`: certificate, quorum, `AggWithinHonestRange`.
- ✅ 18. Mô hình TLA+ của health certificate: quan sát số, weighted median, `AggWithinHonestRange` (Apalache).
- ✅ 23. Evaluation giữ cấu trúc RQ1–RQ4 (tương ứng X1–X4).
- ✅ Tái cấu trúc paper thành 9 section; macro số liệu; tác giả và corresponding author.
- ✅ Sửa lỗi omission: `ProcessProposal` từ chối certificate dưới quorum khi vote extension đã có.
- ✅ Binding certificate với commit round của height trước (`3e48979`).

### Checklist 14 điểm phản hồi của thầy

| # | Điểm | Trạng thái |
|---|---|---|
| 1 | Engram là lớp adaptive trên CometBFT, không phải BFT mới | ✅ |
| 2 | Baseline RQ2 đúng (hard-gated) | ✅ E11 xong (27 trial); RQ2 viết lại chỉ nói BTC/DA; hard-gated là cài đặt của nhóm |
| 3 | RECOVERING→ANCHORED cần DA backfill | ✅ Code, spec và live: 9 lần backfill (120/300/600 s) khớp 100%, control 4/7 giữ, 3/7 mở, 2 phóng đại + 5 giữ |
| 4 | Ai verify ZK proof, vai trò Bitcoin | ✅ |
| 5 | Sửa claim "single recursive proof" | ✅ Chuỗi proof nối bằng root; không recursion |
| 6 | Formal verification không overclaim | ✅ Refinement ghi là còn mở (mục 19) |
| 7 | Formal hoá health certificate | ✅ |
| 8 | Phân biệt core consensus P2P và peripheral P2P | ✅ |
| 9 | Đo sâu RQ4 | 🟡 Tải thấp đủ (round, nil, rejection, committed tx/s, latency, mempool, tx/block; 4 nhánh, n=5); chưa tìm được trần (không nhánh nào bão hòa đến 500 tx/s) |
| 10 | Hysteresis thích nghi | 🟡 Sweep live: adaptive 8,3 flap so với 9,0 của k=2; `StableWindow=64` còn 3,0; static k≥4 là 0; hạ xuống thành phần phân tích (mục 28) |
| 11 | Byzantine cộng tác f=2 live | 🟡 b9 (ngưỡng đúng ở f), b5, b8 xong; suite chính đang chạy lại (mục 13) |
| 12 | Multi-host | 🟡 Hạ tầng xong; chưa có dữ liệu (mục 21) |
| 13 | Related work: cây taxonomy + bảng so sánh | ✅ Cần thầy xem lại độ đủ của literature |
| 14 | Dọn trước khi nộp | 🟡 Tác giả ✅, hình kiến trúc ✅, cụm phòng thủ ✅; còn artifact URL, rà số hình/bảng (mục 24) |
