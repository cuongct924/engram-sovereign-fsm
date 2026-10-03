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
| 11 | Công cụ đo RQ4 | 🟡 | Trần throughput 100→1000 tx/s; chạy lại E7 |
| 12 | Baseline hard-gated (+ soft) | 🟡 | Chạy ma trận E11 (chưa có dữ liệu) |
| 13 | Bộ tấn công cộng tác f=2 (B1–B9) + replay checker | 🟡 | Chạy đủ suite, A1/A2 ở n=7, hình B9 |
| 14 | Làm lại A3, A4, A7, A10 | ⬜ | Xem mục 14 |
| 15 | State root trong checkpoint Bitcoin | 🟡 | Chạy lại E10 trên code cuối |
| 16 | Hysteresis thích nghi ANCHORED↔SUSPICIOUS | 🟡 | Sweep live E5 (tĩnh k=2, k=6, thích nghi) |
| 17 | Dữ liệu DA thật + backfill khoảng Sovereign | 🟡 | Ablation backfill, TLC lớp Server, control "inflated" |
| 18 | Mô hình TLA+ của health certificate | ✅ | — |
| 19 | Refinement C1 | 🟡 | Giữ framing "layered + bounded"; tuỳ chọn chạy tiếp phần consensus-core |
| 20 | Chạy lặp các thí nghiệm live | ⬜ | Giai đoạn 4 |
| 21 | Chạy đa máy có WAN impairment | 🟡 | Hạ tầng sẵn; chưa có dữ liệu |
| 22 | Demo verifier ZK bên ngoài (tuỳ chọn) | ⬜ | Chỉ nếu còn thời gian |
| 23 | Tổ chức Evaluation | ✅ | Giữ RQ1–RQ4 |
| 24 | Hoàn thiện trước khi nộp | 🟡 | Xem mục 24 |
| 25–27 | Việc chờ quyết định và tồn đọng | — | — |

Đã chuyển sang cấu trúc 9 section; paper dùng macro số liệu trong `docs/paper/results/result_macros.tex`.

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
| 2 | **E11** baseline (`e11_hard_gated_baseline/run_matrix.sh`) | Không có (`results_live/` trống) | Chạy mới: hard-gated, engram, soft × lỗi BTC/DA/cả hai × 3 trial; hình height theo thời gian (mục 12) |
| 3 | **E2** S1–S7 (`confidence_interval_recovery_time.py`) | Một lượt cũ | S1–S7 × 5, đảo thứ tự S2–S6, thêm S5 độc lập; khoảng 2–2,5 giờ |
| 4 | **E9** composite (`e9_composite_fault/`) | Một lượt | Composite × 5, P4 × 3 |
| 5 | **E3** (`e3_failure_matrix/`) | Backfill 120/300/600 s, negative control (4/7 giữ, 3/7 mở) | Chạy lại trên code cuối; thêm control "hai validator báo contiguous height phóng đại" (chưa có dữ liệu) |
| 6 | **E5** (`e5_hysteresis_flapping/`) | Sweep tĩnh; không có nhánh thích nghi | Sweep k=2, k=6, thích nghi (k0=2, k_max=8, W=100), 3 trial, hai chế độ (mục 16) |
| 7 | **E8** (`e8_attack_resilience/live_collusion_suite.py`) | Một trial B1–B5 (suite chưa xong) | B1–B9 đủ (B1–B7 3 trial, B8 1 trial, B9 5 trial); A1/A2 ở n=7; A3/A4/A7/A10 (mục 14); A11 round-replay |
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

### 12. Baseline hard-gated (🟡) [T][B]
Chế độ đã có: `engramd start --hard-gated|--soft` hoặc `ENGRAM_MODE`, `make testnet-up MODE=…`,
`scripts/e11_hard_gated_baseline/run_matrix.sh`. Chưa có dữ liệu. Cần chạy: mỗi trial 60 s khoẻ → 180 s
lỗi → 120 s phục hồi; lỗi = BTC tạm dừng, DA outage (cả 7 bridge), cả hai; 3 trial × 3 lỗi × 3 hệ, genesis
mới cho mỗi hệ. Metric: block commit trong cửa sổ lỗi, khoảng lặng dài nhất, số round ở height bị kẹt,
thời gian ra block đầu tiên sau phục hồi, số withdrawal không an toàn. Kết quả điền vào bảng RQ2 và
hình height theo thời gian.

### 13. Bộ tấn công cộng tác f=2 (🟡) [T][M][B]
Code (hook Byzantine, replay checker `engramd debug replay-fsm`, `live_collusion_suite.py`, test
in-process B1–B3, B5, B6, B9) đã có. Dữ liệu live hiện có là một trial B1–B5 trước round binding. Cần chạy đủ:
- B1–B7 × 3 trial, B8 × 1 trial (≥ 10 phút), B9 sweep b ∈ {0, 1, 2, 3} × 2 mục tiêu × 5 trial.
- A1/A2 (eclipse, Sybil) ở n=7; A11 round-replay ở live.
- Hình B9: trục x = b, trục y = tỷ lệ height do Byzantine kiểm soát, tô vùng b ≤ f.
- Replay checker cho mỗi case; paper báo số dòng vi phạm (kỳ vọng 0; b = 3 là đối chứng âm).
- B7/B8 chỉ đo, không khẳng định chuyển trạng thái; n=1 (B8) và n=3/5 chỉ đủ báo số lần đạt.

### 14. Làm lại A3, A4, A7, A10 (⬜) [M]
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
- Control "hai validator ký contiguous height phóng đại không mở được cổng": chưa có dữ liệu.
- Viết kết quả negative control vào `evaluation` và bỏ `\pending` tương ứng.
- Chạy lại toàn bộ trên code cuối (Giai đoạn 4).

### 19. Refinement C1 (🟡) [T]
Đã làm: tách làm hai phần. Phần môi trường hoàn tất (4.826.336 trạng thái, không vi phạm). Phần
consensus-core chưa hội tụ (lần chạy tốt nhất tới depth 12, không vi phạm). Paper giữ framing "layered
specification + bounded model checking" và nêu refinement còn mở. Nếu muốn tiếp tục: chạy
`MC_ConsensusCoreRefinementSafety` lâu hơn hoặc thử kiểm tra quy nạp bằng Apalache cho bước (b)
(`EngramServerRefinement_Ind.tla`: `IndInv`, biến ghost `prevAbs`). Điều kiện dừng: nếu vẫn mở, giữ
framing hiện tại.

### 21. Chạy đa máy có WAN impairment (🟡) [T][M]
Hạ tầng đã có: `deploy/` (Ansible + Terraform cho AWS, GCP, DigitalOcean) và backend SSH của
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
| 2 | Baseline RQ2 đúng (hard-gated) | 🟡 Tooling xong; chạy ma trận E11 (mục 12) |
| 3 | RECOVERING→ANCHORED cần DA backfill | ✅ Code và spec; kiểm chứng live ở mục 17 |
| 4 | Ai verify ZK proof, vai trò Bitcoin | ✅ |
| 5 | Sửa claim "single recursive proof" | ✅ Chuỗi proof nối bằng root; không recursion |
| 6 | Formal verification không overclaim | ✅ Refinement ghi là còn mở (mục 19) |
| 7 | Formal hoá health certificate | ✅ |
| 8 | Phân biệt core consensus P2P và peripheral P2P | ✅ |
| 9 | Đo sâu RQ4 | 🟡 Tooling xong; trần throughput (mục 11) |
| 10 | Hysteresis thích nghi | 🟡 Cài đặt xong; sweep live (mục 16) |
| 11 | Byzantine cộng tác f=2 live | 🟡 Mục 13 |
| 12 | Multi-host | 🟡 Hạ tầng xong; chưa có dữ liệu (mục 21) |
| 13 | Related work: cây taxonomy + bảng so sánh | ✅ Cần thầy xem lại độ đủ của literature |
| 14 | Dọn trước khi nộp | 🟡 Tác giả ✅, hình kiến trúc ✅, cụm phòng thủ ✅; còn artifact URL, rà số hình/bảng (mục 24) |
