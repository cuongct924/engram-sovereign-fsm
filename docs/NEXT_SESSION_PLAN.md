# Kế hoạch công việc: paper và source code

Danh sách việc còn lại, xếp theo thứ tự nên làm: việc đứng trước làm trước. Mục tiêu cuối cùng
là paper `docs/paper/` sẵn sàng nộp: không còn `\pending{...}` nào
(`grep -rn pending docs/paper/sections/` trả về rỗng) và mọi claim khớp với bằng chứng.

Nguồn yêu cầu, ghi ở từng mục:

| Ký hiệu | Nguồn |
|---|---|
| **[T]** | Feedback của thầy (định vị đóng góp, threat model, baseline, refinement, thu gọn thí nghiệm) |
| **[M]** | Review phương pháp luận (cách đo, cách báo cáo, tái lập) |
| **[B]** | Review blind "Major Revision" (research gap, liveness của cái gì, ZK) |

Quy tắc chung:
- Mọi thay đổi giao thức đi theo thứ tự spec TLA+ → Go → TLC/Apalache → unit test → chạy live
  (skill `go-spec-fidelity`).
- Mọi đoạn viết mới trong paper theo skill `paper-writing` (mục VI về mức độ claim, mục X checklist trước khi nộp).
- Thí nghiệm cuối (mục 20–21) chỉ chạy **một lần**, sau khi các thay đổi giao thức (mục 15–17) xong.

## Bảng tổng quan

| # | Việc | Giai đoạn | Đổi giao thức | Công sức | Phụ thuộc |
|---|---|---|---|---|---|
| 1 | Chốt các quyết định | 0 | — | 0,5 ngày | — |
| 2 | Đặt trước tiêu chí thành công cho từng RQ | 1 | không | 0,5 ngày | 1 |
| 3 | Điều tra bất thường reorg (cờ critical không bật) | 1 | không | 0,5 ngày | — |
| 4 | Phân tích lại dữ liệu đã có | 1 | không | 0,5–1 ngày | 2 |
| 5 | Viết security semantics theo mode + định lý withdrawal | 1 | không | 1 ngày | — |
| 6 | Viết lại lập luận lõi: vì sao consensus, liveness của cái gì, ZK phục vụ ai | 1 | không | 1–2 ngày | 5 |
| 7 | Reproducibility + định nghĩa chỉ số | 1 | không | 1–2 ngày | — |
| 8 | Áp dụng hệ nhãn bằng chứng + gọn phần giới hạn | 1 | không | 0,5 ngày | — |
| 9 | Sửa `spec/README.md` còn lại | 1 | không | 0,5 ngày | — |
| 10 | Smoke test live bản sửa sub-quorum certificate | 2 | không | 0,5 ngày | — |
| 11 | Công cụ đo RQ4 (round, committed throughput, độ trễ tx) | 2 | không | 1–2 ngày | — |
| 12 | Baseline hard-gated (+ soft/optimistic) | 2 | chế độ baseline | 1–2 ngày | 1, 11 |
| 13 | Bộ tấn công cộng tác f=2 (B1–B8) + sweep Byzantine b = 0…3 (B9) + replay checker (code xong, đang chạy live) | 2 | hook Byzantine | 4–5 ngày | 11 |
| 14 | Làm lại A3, A4, A7, A10 để phòng thủ thực sự kích hoạt | 2 | không | 1–2 ngày | 13 |
| 15 | State root trong checkpoint Bitcoin | 3 | có | 1 ngày | — |
| 16 | Hysteresis thích nghi ANCHORED↔SUSPICIOUS | 3 | có | 3 ngày | 4 |
| 17 | Dữ liệu DA thật + backfill khoảng Sovereign | 3 | có | 4–5 ngày | — |
| 18 | Mở rộng mô hình TLA+ của health certificate | 3 (song song) | chỉ spec | 3–5 ngày | — |
| 19 | Refinement C1 (chia hai bước / quy nạp) | 3 (song song) | chỉ spec | 1–2 tuần, không chắc | 1 |
| 20 | Chạy lặp các thí nghiệm live | 4 | không | ~1 ngày máy | 15–17 |
| 21 | Chạy đa máy có WAN impairment | 4 | không | 3–5 ngày | 1, 12, 15–17 |
| 22 | Demo verifier ZK bên ngoài (tuỳ chọn) | 4 | không | 2–3 ngày | 6, 15 |
| 23 | Tái cấu trúc Evaluation thành X1–X4 | 5 | không | 1–2 ngày | 20–21 |
| 24 | Hoàn thiện paper trước khi nộp | 5 | không | 1 ngày | tất cả |
| 25–27 | Việc chờ quyết định và tồn đọng | — | — | — | — |

## Đã xong (không cần làm lại)

- Sửa lỗi omission: `ProcessProposal` từ chối certificate dưới 2f+1 khi vote extension đã có
  (`voteExtensionsAvailableAt`, `x/sovereignty/proposal.go`; commit `fcdc42b`).
- Paper: định vị Engram là lớp thích nghi trên CometBFT, không phải BFT mới; tách core network và
  peripheral network; health certificate + Lemma 1 (§3.3); sửa claim ZK (chaining, không recursion;
  validator verify, Bitcoin chỉ timestamp); related work viết lại có cây taxonomy và bảng so sánh
  (commit `84a0d00`, `5ba7580`).
- Paper: đã sửa −22% (thay "~23–27%", "modest"), "p95 block interval" (thay "commit latency"),
  "Tx accepted to mempool/s", bounded model checking ghi rõ bound, phạm vi con số 3,39 triệu
  trạng thái, caption hình reorg.
- `spec/README.md`: bỏ theorem language (Theorem 7.1/8.1 → Target Property, Lemma 7.2–7.6 →
  Property), thêm bảng trạng thái Checked/Open/Partial/By construction ở đầu §7.
- Health certificate đã có mô hình TLA+ dạng Boolean ở lớp server (commit `1892bde`).

---

## Checklist: 14 điểm phản hồi mới, đối chiếu hiện trạng

- [x] 1. Định vị Engram là lớp adaptive trên CometBFT, không phải BFT agreement mới —
  `introduction.tex:32-43` đã nêu rõ proposer selection/prevote/precommit/quorum intersection vẫn
  do CometBFT cung cấp.
- [ ] 2. Baseline RQ2 đúng (hard-gated, chặn block khi thiếu BTC/DA receipt) thay cho vanilla
  CometBFT — chưa làm, trùng mục #12.
- [ ] 3. Điều kiện RECOVERING→ANCHORED cần thêm "dữ liệu Sovereign interval đã republish lên DA" —
  `CalculateNextState` (`circuit_breaker.go:82-96`) hiện chỉ kiểm `SafeBlocks`+`ReanchoringProofValid`,
  chưa có điều kiện DA; trùng mục #17b.
- [x] 4. Rõ ai verify ZK proof và vai trò Bitcoin — `introduction.tex:73-80` đã ghi "Validators
  verify each proof inside block execution; Bitcoin timestamps... but does not verify the proof".
- [ ] 5. Sửa claim "single recursive proof" — wording đã đổi thành chuỗi proof nối bằng root
  (`introduction.tex:76-78`), nhưng root đó mới nằm trong state Engram (`LastAnchoredRoot`), chưa
  được ghi vào OP_RETURN Bitcoin thật; trùng mục #15.
- [ ] 6. Formal verification không overclaim — `formal_verification.tex` đã đánh dấu refinement
  "(open)" kèm `\pending`, đã có đoạn giải thích `QuorumOverlap`/LiDO + 4 giả định fairness (message
  delivery, timeout, proposer, FSM progress); nhưng bản thân refinement C1 vẫn chưa hội tụ, trùng
  mục #19.
- [ ] 7. Formal hoá health certificate đầy đủ (trường số, stale/missing/conflicting observation,
  honest validator bị eclipse) — mới có mô hình Boolean (commit `1892bde`), phần số/weighted-median
  và honest-eclipse chưa có; trùng mục #18.
- [x] 8. Phân biệt core consensus P2P (phải giữ quorum) và peripheral P2P degradation —
  `introduction.tex:45-55` đã viết rõ, dùng nhất quán "P2P degradation" theo nghĩa peripheral.
- [ ] 9. Đo sâu RQ4 (round/height, nil-vote, lý do reject, committed tx/s, latency p50/p95, mempool
  backlog, tx/block) — chưa có công cụ đo, trùng mục #11.
- [ ] 10. Hysteresis thích nghi (tăng ngưỡng sau flap liên tiếp, giảm khi ổn định) — chưa làm,
  trùng mục #16.
- [ ] 11. Byzantine cộng tác f=2 (N=7) live — code và replay checker xong, suite live đang chạy
  (2026-10-02); chưa có số liệu vào paper; trùng mục #13.
- [ ] 12. Multi-host experiment (nhiều máy/VM thật) — chưa làm, trùng mục #21.
- [ ] 13. Related work: cây taxonomy + bảng so sánh theo tiêu chí ("external health agreed by
  quorum", "in-consensus fallback", v.v.) đã có (`related_work.tex`, 60 `\cite`); "bổ sung thêm
  literature" là yêu cầu định tính — chưa có cách tự xác nhận đã đủ, cần thầy xem lại trực tiếp.
- [ ] 14. Dọn trước khi nộp:
  - [x] Bảng FSM states đã tách cột BFT finality / BTC settlement riêng (`system_design.tex:195`).
  - [ ] Figure 1 (architecture) — label `fig:architecture` tồn tại, chưa có `\includegraphics`,
    ảnh chưa làm.
  - [ ] Placeholder tác giả/affiliation — `main.tex:37-38` còn `TODO` và "Author One/Two".
  - [ ] Rà số thứ tự figure/table toàn bài — chưa kiểm tra, gộp vào mục #24.
  - [x] Cụm "we honestly report / explicitly state / we do not claim" — đã rà `sections/*.tex`,
    không còn xuất hiện.

---

## Giai đoạn 0 — Chốt quyết định

### 1. Chốt các quyết định

Các quyết định này chặn nhiều mục phía sau; chốt trước khi bắt đầu giai đoạn 2.

| # | Câu hỏi | Đề xuất | Ảnh hưởng |
|---|---|---|---|
| 1a | Deadline và venue dự kiến | — | Có làm kịp mục 19, 21, 22 hay không |
| 1b | Hạ tầng đa máy | CloudLab/Chameleon/Emulab (miễn phí cho nghiên cứu, máy vật lý); hoặc cloud nhiều vùng; hoặc máy lab + netem | Mục 21 |
| 1c | Thêm baseline soft/optimistic | Có | Mục 12 |
| 1d | Baseline hard-gated khi BTC tạm dừng: giới hạn phần `round` trong tolerance hay giữ nguyên | Giới hạn (`round` → `min(round, 3)`) | Mục 12 |
| 1e | Mức tham vọng formal: refinement hai bước, hay giữ framing "layered + bounded" và chỉ sửa wording | Thử hai bước, dừng sau 2 tuần | Mục 19 |
| 1f | Thứ tự 5 contribution [B] | Giữ 5, đưa đóng góp lõi (health certificate + semantics theo mode) lên đầu, formal và ZK xuống cuối | Mục 6, 23 |
| 1g | Mốc dùng phương án dự phòng cho RQ1/RQ2 nếu mục 12–13 không kịp [B] | Ngày cụ thể, cần chốt | Mục 23 |

Phương án dự phòng (1g): RQ2 viết thành "degrade an toàn" (chain tiếp tục, withdrawal khoá đúng
lúc, không lệch AppHash, replay 0 vi phạm), không claim "cải thiện liveness" so với baseline; RQ1
thu hẹp về đúng phạm vi đã test (single-actor), chuyển collusion sang limitations.

---

## Giai đoạn 1 — Làm ngay, không cần chạy thí nghiệm mới

### 2. Đặt trước tiêu chí thành công cho từng RQ [M]

Làm **trước** mọi lượt chạy mới. Ghi vào `docs/EXPERIMENT.md` và commit trước khi chạy (để có mốc
thời gian trong git); paper dùng đúng các ngưỡng này, và mọi từ "correctly/bounded/acceptable/
cadence held" phải gắn với một ngưỡng hoặc bị bỏ.

Mỗi RQ gồm: metric, ngưỡng đạt/không đạt, oracle, số lần lặp. Dự thảo (các con số là đề xuất,
cần chốt):

| RQ | Metric | Tiêu chí đạt |
|---|---|---|
| RQ1 | Vi phạm replay (mục 13) mỗi case B1–B8 | 0 vi phạm trên mọi trial |
| RQ1 | Sweep B9, b ≤ 2 | 0 height có aggregate ngoài khoảng honest; 0 withdrawal không an toàn |
| RQ1 | Sweep B9, b = 3 (f+1) | Đối chứng âm: vi phạm xuất hiện, và chỉ ở height có proposer Byzantine |
| RQ1 | AppHash divergence | 0 height lệch |
| RQ2 | Block trong cửa sổ lỗi, Engram vs. hard-gated | Engram ≥ 80% số block lúc khoẻ cùng thời lượng; hard-gated = 0 khi DA outage |
| RQ2 | "Không stall" | Không có khoảng giữa hai block > 3 × block interval lúc khoẻ |
| RQ3 | Số flap mỗi giờ khi không có lỗi | ≤ 1 flap/giờ ở WAN profile chuẩn |
| RQ3 | Độ trễ phục hồi sau khi lỗi hết | ≤ hysteresis wait + thời gian prove (khai báo con số) |
| RQ4 | Giảm block rate so với vanilla | Báo CI 95% của hiệu số; "acceptable" = ngưỡng khai báo trước (ví dụ ≤ 25%) |
| Attack | "Cadence held" | Block rate giảm < 10% so với trước tấn công |

- Oracle cho S1–S7: bảng trạng thái FSM kỳ vọng cho từng kịch bản (ví dụ S5 → chỉ SUSPICIOUS
  thoáng qua, không SOVEREIGN), viết trước khi chạy lại.
- Tiêu chí chọn thí nghiệm cho main body: thí nghiệm trả lời trực tiếp một RQ → main body; còn lại
  → appendix. Ghi thành một câu ở đầu §6.

### 3. Điều tra bất thường reorg [B]

Appendix nói kiểm tra lại anchor "fires at every tested depth ≥ 2" nhưng FSM chỉ phản ứng khi gap
vượt ngưỡng. Điều này mâu thuẫn với code: `IsBtcSpvFailed` nằm trong `IsCriticalCondition`
(`x/sovereignty/types/predicates.go:44`); nếu cờ này lên `true` trong certificate, FSM phải nhảy
thẳng SOVEREIGN. Thực tế reorg sâu 2–5 giữ ANCHORED, còn sâu 15 đi qua SUSPICIOUS trước (do gap,
không do critical).

Kiểm tra bằng log E10 (`scripts/e10_bitcoin_reorg/results_live/`) ba giả thuyết:
1. Cờ chỉ bật ở một thiểu số validator → weighted majority của certificate vẫn `false`. Nếu đúng,
   đây là ví dụ thực tế của Lemma 1 và nên đưa vào paper.
2. Cờ bật thoáng qua giữa hai block (anchor được submit lại trong tolerance) nên không vào vote
   extension nào.
3. Cờ không bao giờ bật — câu trong appendix sai.

Sửa appendix theo kết quả.

### 4. Phân tích lại dữ liệu đã có [M]

Không cần chạy mới; dùng định nghĩa chỉ số của mục 7 và tiêu chí của mục 2.
- Đếm flap trực tiếp từ log 153 transition của S1–S7 (`scripts/e2_fault_injection/results_live/`)
  và từ log composite/P4. Dùng số liệu này giải quyết mâu thuẫn giữa sweep RQ3 (ngưỡng 2 flap dưới
  WAN) và các thí nghiệm khác chạy ở ngưỡng 2.
- Bảng time-to-detect và time-to-recover cho từng kịch bản S1–S7.
- Composite: báo block rate thực tế — 188 block trong 341 s = 1,81 s/block so với 1,34 s lúc khoẻ
  (chậm ~35%); phân tích chậm ở giai đoạn nào của timeline lỗi.
- Hysteresis sweep in-process: chạy lại nhiều seed (ví dụ 10) để có phương sai; ghi mẫu số của
  absorption rate (94,74% ≈ 18/19 cho thấy mẫu số rất nhỏ — cần run dài hơn).
- RQ3 vế "không làm chậm phục hồi": thêm cột độ trễ phục hồi theo ngưỡng vào bảng hysteresis.
- Đưa số liệu live spot-check của sweep hysteresis vào bảng, không chỉ trong hình.

### 5. Security semantics theo mode + định lý withdrawal [T]

Viết thành một subsection mới trong §3 (sau Threat Model). Định nghĩa bốn tính chất:

| Tính chất | Ý nghĩa | Giả định |
|---|---|---|
| P_agree | Không có hai block/state xung đột | BFT, f < n/3 |
| P_settle | Prefix đã checkpoint không bị viết lại | Bitcoin reorg ≤ K, checkpoint chứa state root (mục 15) |
| P_avail | Dữ liệu mọi block đã commit lấy lại được từ DA | DA honest, publish dữ liệu thật (mục 17) |
| P_exit | Chỉ rút tiền từ prefix thoả P_settle ∧ P_avail | circuit breaker |

| Mode | P_agree | P_settle | P_avail | Withdrawal |
|---|---|---|---|---|
| ANCHORED | ✓ | ✓ | ✓ | mở |
| SUSPICIOUS | ✓ | ✓ (trễ) | ✓ | mở |
| SOVEREIGN | ✓ (chỉ khi f < n/3) | ✗ | ✗ (chỉ ở block store của validator) | khoá |
| RECOVERING | ✓ | chờ proof | đang backfill | khoá |

- **Định lý trung tâm** (chứng minh trên giấy): mọi withdrawal đã commit đều thuộc một prefix thoả
  P_settle ∧ P_avail. Liveness đến từ việc nới receipt; an toàn cho người dùng đến từ việc gắn
  withdrawal với settle + available.
- Ô P_settle và P_avail của ANCHORED chỉ đúng sau mục 15 và 17; trước đó ghi là mục tiêu thiết kế.
- Bổ sung vào threat model:
  - Adversary theo lớp: làm suy giảm liên kết ngoại vi của mọi validator được, nhưng không chia cắt
    core network dưới 2f+1.
  - Byzantine vượt f: ở ANCHORED, checkpoint có root cho phép phát hiện fork (accountability); ở
    SOVEREIGN không có gì bảo vệ — đây là lý do phải khoá withdrawal.
  - Bảo mật của việc chuyển mode: chỉ đổi khi có quorum (Lemma 1); bề mặt tấn công còn lại là
    griefing (mục 6).
- Có thể gộp vào contribution thứ nhất (xem 1f).

### 6. Lập luận lõi: vì sao consensus, liveness của cái gì, ZK phục vụ ai [B]

**6a. Vì sao quyết định phải nằm trong consensus** (một đoạn ở Intro hoặc đầu §3):
- Circuit breaker và việc nới receipt thay đổi *validity* của block và *state* (withdrawal có thực
  thi hay không). Nếu mỗi validator tự áp chính sách cục bộ, các validator có view khác nhau sẽ bất
  đồng về validity (nil-vote) hoặc thực thi withdrawal khác nhau (AppHash lệch).
- Dẫn chứng từ chính repo: `PreBlocker` gọi `RefreshMetrics` đưa sensor cục bộ vào `AppHash` đã gây
  consensus failure thật (ghi trong `CLAUDE.md`). Mô tả ngắn, không kể quá trình debug.
- So với escape hatch: cần một chain khác, do user/watcher kích hoạt, sau khi đã stall; không giữ
  được liveness nội chuỗi.
- Tìm sự cố thực tế đã ghi nhận (rollup/app-chain dừng vì DA, sequencer hoặc settlement layer) —
  chỉ đưa vào nếu có nguồn chính thức (post-mortem, status page).

**6b. Discussion theo khung ba trục** (liveness nội chuỗi, an toàn tài sản liên chuỗi, chi phí
griefing), thay cho danh sách giới hạn dài:
- Liệt kê cái gì vẫn chạy ở SOVEREIGN (giao dịch nội chuỗi, logic ứng dụng, forced-tx queue) và cái
  gì dừng (withdrawal liên chuỗi).
- Nói rõ prototype không có bridge hay tài sản thật: app không mount `x/auth`/`x/bank`, tx là
  envelope không ký, withdrawal được nhận diện bằng chuỗi con `TX_WITHDRAWAL` (`containsWithdrawal`,
  `x/sovereignty/proposal.go:271`). Circuit breaker được đánh giá ở mức *chính sách*.
- Phản bác "griefing tạo bề mặt halt mới": cùng một kẻ tấn công làm suy giảm view ngoại vi của
  quorum thì chain hard-gated **dừng toàn bộ**, còn Engram chỉ **đóng băng withdrawal**. Số liệu lấy
  từ mục 12.
- Chi phí griefing: phải làm suy giảm liên kết ngoại vi của ≥ 2f+1 validator (Lemma 1), so với 1
  node trong thiết kế không có quorum.
- Giải thích nhân quả cho từng kết quả bất thường: sụt block rate 22% (mục 11), ngưỡng 2 vs. ≥ 6
  (mục 4, 16), reorg 3–5 (mục 3). Chưa có bằng chứng thì nêu giả thuyết cụ thể kèm cách kiểm chứng.

**6c. ZK proof phục vụ verifier nào** (§3.8):
- QC của CometBFT chỉ chứng minh "một quorum đã ký". Proof chứng minh thêm "những gì đã ký tuân
  thủ luật FSM và withdrawal lock" — đúng cả khi quorum gian lận. Giá trị này chỉ có với một
  **verifier bên ngoài** không tin quorum: bridge/light client ở chuỗi khác, node mới đồng bộ, hoặc
  người quan sát Bitcoin khi checkpoint có root (mục 15).
- Chọn verifier mục tiêu và nói rõ trong §3.8. Demo là mục 22 (tuỳ chọn).
- §Scope của related work vẫn còn đoạn "Relative to the sketch, this paper replaces…": rút còn một
  câu khai báo nguồn gốc; novelty chỉ so với SOTA (`tab:comparison`).

### 7. Reproducibility + định nghĩa chỉ số [M]

Chỉ sửa paper; làm song song với code.

**Artifact và môi trường**
- Repository URL, license, commit/tag đúng với kết quả.
- Phần cứng host: CPU, RAM, OS, phiên bản Docker, tài nguyên cấp cho từng container.
- Phiên bản: commit hash của fork `engram-consensus-core` và danh sách thay đổi so với CometBFT
  upstream v0.39.x; Cosmos SDK v0.54.3; celestia-app/celestia-node; Bitcoin Core; Noir/`bb`;
  TLC/Apalache.
- Bảng tham số `tc netem` cho mọi profile (delay, jitter, loss, correlation) và ma trận WAN 7 vùng;
  định nghĩa "WAN latency baseline".
- Timeline tiêm lỗi cho composite và S1–S7: thời điểm bật/tắt, cường độ, thời lượng (bảng hoặc
  Gantt).
- Bảng tham số FSM dùng trong mỗi thí nghiệm: ngưỡng sensor, chu kỳ polling,
  `MAX_SUSPICIOUS_TIME`, `HysteresisWait`, `DownHysteresisThreshold`, `MaxDownHysteresisThreshold`.

**Định nghĩa chỉ số** (công thức, mẫu số, độ dài run)
- Flapping, absorption rate, Anchored uptime, gap fraction, steady blocks/s.
- Noise profile in-process (phân phối, tham số, seed).
- Monte Carlo P2P: mô hình sinh dữ liệu, "snapshot", "evasion threshold" (đại lượng nào, thang
  0–1), lưới cường độ, seed, số trial lành tính dùng để đo FP.
- Cận trên CI cho tỷ lệ 0% theo rule of three: ~0,15% với 2.000 trial, ~1% với 300 trial. Báo SD/CI
  cho detection delay.

**Formal**
- Bảng CONSTANTS/bound, symmetry reduction, thời gian chạy, phần cứng cho mỗi config.
- Nói rõ "Depth 22/25" là diameter đã duyệt hết (TLC BFS kết thúc) hay giới hạn cứng.
- Bỏ "no false negatives" trong ablation; chỉ nói mỗi ablation tái hiện đúng vi phạm dự đoán.

**Minh bạch và trình bày**
- Sự cố contention "identified and corrected before measurements": sửa gì, dữ liệu trước đó có bị
  loại không, loại bao nhiêu run.
- Bảng attack: tách kết quả 4 node và 7 node (hoặc thêm cột n) và cột tỷ lệ voting power Byzantine.
- Bảng P2P: tiêu đề có "vs." nhưng chỉ một cột → thêm cột peer-count-only.
- Cạnh RECOVERING→ANCHORED: giải thích "uptime 59,8% → 0% là thiết kế" — trong RECOVERING, critical
  đẩy thẳng về SOVEREIGN (`CalculateNextState`, `x/sovereignty/keeper/circuit_breaker.go:82-85`), nên
  chuỗi healthy phải bắt đầu lại; `HysteresisWait` càng lớn càng khó đạt giữa hai lần critical.
- Làm rõ "0,953 blocks/s vs. 1,34 s/block" không mâu thuẫn: 1,34 s là *Engram* ở S1, khớp với
  0,743 blocks/s của Engram trong E7; vanilla chỉ có trong E7.
- Bitcoin congestion: nói rõ là dừng submit checkpoint (mô phỏng hệ quả), không phải nghẽn mempool.
- DA outage: nói rõ là tắt bridge, không phải lỗi sampling.

### 8. Hệ nhãn bằng chứng + gọn phần giới hạn [T][B]

- Áp dụng cho toàn paper; rà mọi chỗ "verified/proves/guarantees":

| Nhãn | Dùng cho |
|---|---|
| Lemma/Theorem | Chứng minh trên giấy đầy đủ (Lemma 1, định lý withdrawal) |
| Property (model-checked, N ≤ 4) | Kết quả TLC/Apalache, luôn ghi bound |
| Target property / Conjecture | Chưa có bằng chứng |
| Observed | Kết quả live |

- Trình bày giới hạn như ranh giới của một kết quả đã vững: mỗi giới hạn nói một lần ở chỗ của nó,
  các chỗ khác tham chiếu; bỏ lặp giữa Setup, Discussion, Conclusion.

### 9. Sửa `spec/README.md` còn lại

1. §7.4 "Threat Model Boundary: Sensor View Homogeneity": rút bullet "Fourth precondition" về sự
   thật hiện tại — certificate đã có trong code và đã có mô hình TLA+ dạng Boolean (xem §6.2); phần
   cơ chế và lý do thiết kế thuộc về §6.2.
2. §6.2 "Proposal Validation": thêm rằng certificate dưới 2f+1 bị từ chối khi vote extension đã có.
3. Future Work "Sensor Aggregation Pre-Consensus Phase": chỉ còn mở phần mô hình số và
   validator trung thực bị eclipse (mục 18).

---

## Giai đoạn 2 — Code không đổi giao thức + thí nghiệm

### 10. Smoke test live bản sửa sub-quorum certificate

Unit test chỉ phủ `ProcessProposal`; luồng PrepareProposal → CometBFT → ProcessProposal chưa chạy
live. Nếu proposer trung thực tạo certificate thiếu quorum, mọi validator giờ từ chối nó — một
nguyên nhân hệ thống sẽ làm chain dừng thay vì vào SOVEREIGN.

1. Xoá `testnet-data/` + `data/bitcoin-node0N/` + `data/celestia-*/`, chạy `make testnet-up`.
2. Chạy khoẻ 30 phút. Đạt khi:
   - `docker logs engram-node0N 2>&1 | grep -c 'reason=sub_quorum_certificate'` bằng 0 trên mọi node.
   - Block rate như chế độ extended của E7 (~0,74 block/s).
3. Chạy S3 (DA outage) một lần để xác nhận nhánh degraded vẫn đúng.
4. Tấn công omission là case B3 (mục 13).

### 11. Công cụ đo RQ4 [T][M]

Mục tiêu: điền các dòng dưới của `tab:overhead` và xác nhận (hoặc bác bỏ) rằng mức sụt block rate
22% đến từ các consensus round thêm. Mọi số liệu lấy từ cluster đang chạy, không sửa code chain.

| Metric | Nguồn |
|---|---|
| Round mỗi height | `/commit?height=h` → `signed_header.commit.round` (+1) |
| Số proposal bị từ chối mỗi height, theo lý do | log `ProcessProposal REJECT height=… reason=…` |
| Nil prevote / precommit | websocket `/subscribe` `tm.event='Vote'`; nil = `block_id.hash` rỗng |
| Committed tx/s | `/block?height=h` → `len(data.txs) - 1` (Txs[0] là certificate; vanilla không trừ 1) |
| Độ trễ commit tx p50/p95 | thời điểm submit theo tx hash (sha256 của tx bytes) so với `header.time` của block chứa nó |
| Backlog mempool | poll `/num_unconfirmed_txs` mỗi 0,5 s |
| Tx mỗi block, kích thước tx | cùng lần quét `/block` |

Triển khai:
- Mới: `scripts/e7_consensus_overhead/consensus_metrics.py` — quét dải height (round, số tx, thời
  gian block), thu vote qua websocket (chạy trong thread khi đang tải), phân tích lý do từ log. Đo
  **mọi** height (websocket `NewBlock` hoặc quét `/block` sau khi chạy) — poll hiện ~2,7 s/tick,
  chậm hơn block (~1,3 s), không đủ để khẳng định "no stall". Các mục 12, 13, 20 dùng lại công cụ
  này.
- `live_throughput_latency.py`: thêm `--deep-metrics`, ghi `results_live/deep_<ts>.csv` + summary.
- Chạy: 20 tx/s, `--repeats 5`, cả hai chế độ; sau đó tăng 100→200→500→1000 tx/s để tìm trần
  committed throughput.
- Microbenchmark 18,25 µs: báo số lần lặp và phương sai (`-count=10`, `benchstat`); ghi rõ không gồm
  verify proof (~26 ms, đo riêng). Thay Cohen's d bằng CI 95% của hiệu số blocks/s.
- Phân tích: nếu round/height của Engram ≈ 1/0,743 còn vanilla ≈ 1/0,953 thì lập luận đúng;
  histogram lý do từ chối chỉ ra nguyên nhân cụ thể. Cập nhật abstract, §6.4 và conclusion.

### 12. Baseline hard-gated (+ soft/optimistic) [T][B]

"Tương đương" = cùng binary, cùng phần cứng, cùng lịch tiêm lỗi, chỉ khác một chính sách. Ba hệ tạo
thành trade-off hai chiều:

| Hệ | Liveness khi lỗi | Withdrawal an toàn |
|---|---|---|
| Hard-gated (luôn bắt buộc receipt) | ✗ dừng | ✓ |
| Soft/optimistic (bỏ qua receipt khi lỗi, không khoá) | ✓ | ✗ |
| Engram | ✓ | ✓ |

Metric mới: **số withdrawal không an toàn đã commit** (rút từ prefix chưa settle/available). Tuỳ
chọn baseline thứ tư: operator can thiệp tay (dừng, sửa config, khởi động lại), đo thời gian gián
đoạn.

Triển khai (chế độ baseline giống `--vanilla`, không đổi spec; ghi rõ như vậy trong paper):
1. `cmd/engramd/main.go:374-382`: thêm `--hard-gated` và `--soft` (loại trừ lẫn nhau và với
   `--vanilla`), truyền qua `app.NewEngramApp` vào `NewPrepareProposalHandler` /
   `NewProcessProposalHandler` / `CommitFSMTransition`.
2. Hard-gated:
   - `PrepareProposal`: target state luôn `ANCHORED` (bỏ qua `CalculateNextState`).
   - `ProcessProposal`: state kỳ vọng là `ANCHORED`; gọi `da.VerifyReceipt` / `anchor.VerifyReceipt`
     với `fsmState = ANCHORED` để `requiresCheck` luôn đúng.
   - Không circuit breaker (withdrawal luôn được phép).
3. Soft: như Engram nhưng không khoá withdrawal ở SOVEREIGN/RECOVERING.
4. **Tolerance:** `anchor.Tolerance = kDeepFinality + livenessMargin + round` (`x/anchor/verify.go:30`)
   tăng không giới hạn, nên khi BTC tạm dừng, chain hard-gated cuối cùng vẫn chấp nhận sau đủ nhiều
   round (chậm dần, không dừng hẳn). `da.Tolerance` giới hạn ở 4 (`x/da/verify.go:16`), nên DA
   outage là dừng thật. Làm theo quyết định 1d và ghi trong §5.3.
5. Docker: env `ENGRAM_MODE=hard-gated|soft` trong `docker/engram-validator-cluster.yml`;
   `make testnet-up MODE=…`.
6. Test trong `proposal_test.go`: hard-gated từ chối proposal thiếu DA receipt hợp lệ kể cả khi
   certificate báo DA không khoẻ.

Thí nghiệm `scripts/e11_hard_gated_baseline/live_hard_gated_vs_engram.py`:
- Mỗi trial: 60 s khoẻ → 180 s lỗi → 120 s phục hồi; lỗi = BTC tạm dừng, DA outage (cả 7 bridge),
  cả hai. 3 trial × 3 lỗi × 3 hệ, genesis mới cho mỗi hệ.
- Metric: block commit trong cửa sổ lỗi, khoảng lặng dài nhất giữa hai block, số round ở height bị
  kẹt (`/dump_consensus_state`), thời gian ra block đầu tiên sau phục hồi, số withdrawal không an
  toàn (gửi withdrawal định kỳ trong suốt trial).
- Kết quả `results_live/*.csv`, summary `.md`, hình height theo thời gian cho cả ba hệ, tô vùng lỗi
  (hình mới cho §6.2).

### 13. Bộ tấn công cộng tác f=2 (B1–B9) + replay checker [T][M][B]

Mục tiêu: RQ1 ở đúng quy mô threat model (n=7, f=2), với kiểm tra tính đúng mạnh hơn AppHash
agreement. Chạy lại A1/A2/A6 ở n=7.

**13a. Hạ tầng hai node Byzantine**
- Hiện chỉ có `docker/engram-node04-byzantine.yml`. Tổng quát hoá thành
  `ENGRAM_BYZANTINE_NODES=node04,…` (B9 cần tới 3 node: node04–node06) thay vì thêm từng file compose.
- `ENGRAM_BYZANTINE_BEHAVIOR` hiện chỉ tác động phía proposer (`applyByzantineBehavior`,
  `proposal.go`). Thêm:

| Giá trị | Hook | Tác dụng |
|---|---|---|
| `false_observation:<json>` | `NewExtendVoteHandler` (`vote_extension.go`) | ghi đè trường trong metric đã ký, ví dụ `{"btc_gap":<SovereignThreshold>,"is_das_failed":true}` |
| `bias_certificate` | `PrepareProposal` | giữ đúng quorum, ưu tiên giá trị cực đoan (mình + đồng phạm + honest xấu nhất) |
| `omit_certs:<k>` | `PrepareProposal` | chỉ giữ `k` certificate |
| `stale_receipt` | `PrepareProposal` | dùng lại receipt DA/BTC của 50 height trước |

**13b. Các case**

| Case | node04 | node05 | Kỳ vọng |
|---|---|---|---|
| B1 | `false_observation` | `false_observation` | không chuyển trạng thái (Lemma 1) |
| B2 | `false_observation` | `bias_certificate` | aggregate nằm trong khoảng honest; không chuyển khi honest khoẻ |
| B3a | `omit_certs:5` (= quorum) | honest | chấp nhận, aggregate trong khoảng honest |
| B3b | `omit_certs:2` | honest | từ chối `sub_quorum_certificate`, round sau commit |
| B4 | double-sign | double-sign | có evidence cho cả hai, commit trong 1–2 block |
| B5 | withdrawal ở SOVEREIGN | withdrawal ở SOVEREIGN | không bao giờ commit |
| B6 | `stale_receipt` | `forge_btc_hash` | bị chặn bởi kiểm tra receipt |
| B7 | báo suy giảm | báo suy giảm | + 2 honest bị eclipse thật: 4/7 < 2f+1 → không chuyển; biến thể 5/7 → phải chuyển (biên thật của cổng quorum) |
| B8 | honest | honest | Split view thuần honest, không Byzantine: 3/7 rồi 4/7 honest bị eclipse thật (thấy suy giảm) trong ≥ 10 phút. Đo round/height, nil-vote, số lần chuyển trạng thái; kỳ vọng: aggregate luôn nằm trong khoảng honest, không lệch AppHash, số flap ≤ ngưỡng mục 2 |
| B9 | sweep số Byzantine b = 0…3 | — | Đo biên threat model, xem bảng dưới |

**B9 — Sweep số Byzantine b ∈ {0, 1, 2, 3} (đo biên threat model).** Với n = 7 thì
f = ⌊(n−1)/3⌋ = 2 và quorum = 2f+1 = 5; b = 3 = f+1 là một điểm vượt ngưỡng, đủ để chứng minh
ngưỡng là chặt. Không chạy b ≥ 4: mọi giao thức BFT đều mất đảm bảo ở đó, kết quả biết trước và
không nói gì riêng về Engram. Byzantine vẫn bỏ phiếu
consensus bình thường (để chain còn chạy và cô lập tác động lên certificate); chỉ nói dối trong
observation, và khi làm proposer thì `bias_certificate`. Hai mục tiêu tấn công, mỗi mục tiêu một
lượt:
- **Ép SOVEREIGN** khi mọi peripheral khoẻ (griefing: đóng băng withdrawal).
- **Che giấu outage thật:** tắt toàn bộ DA bridge, Byzantine báo khoẻ → nếu chain giữ ANCHORED thì
  withdrawal vẫn mở trong lúc DA chết = **withdrawal không an toàn** (metric của mục 12). Đây là
  kịch bản nguy hiểm nhất.

| b | Vị trí so với ngưỡng (n=7, f=2, quorum 5) | Kỳ vọng |
|---|---|---|
| 0–2 | ≤ f (trong threat model) | Không ép được, không che được (Lemma 1) |
| 3 | f+1 | Ép/che được **chỉ khi proposer là Byzantine** (chọn 3 Byzantine + 2 honest); proposer honest đưa đủ 7 observation thì 4 honest > 7/2, median vẫn honest-bounded |

- Metric mỗi b: tỷ lệ height có aggregate nằm ngoài khoảng honest, có/không chuyển trạng thái bị ép,
  số withdrawal không an toàn (lượt che giấu outage), tỷ lệ height do proposer Byzantine đề xuất.
- B1–B8 kiểm tra an toàn trong ngưỡng (≤ f). B9 với b = 3 là đối chứng âm (kỳ vọng vi phạm): paper
  báo để chứng minh ngưỡng là **chặt** và làm rõ hệ quả khi vượt threat model, không phải để claim
  an toàn. Hình đề xuất: trục x = b, trục y = tỷ lệ height bị Byzantine kiểm soát, tô vùng b ≤ f.
- B8 phủ giới hạn "split view gần ngưỡng quorum chưa được test" mà Discussion cũ đã nêu, và là một
  nguồn khả dĩ của round thêm (đối chiếu với mục 11).

Dùng lại `live_double_signing_test.py` (B4), `live_withdrawal_test.py` (B5),
`live_byzantine_attacks.py` (B6); mới `scripts/e8_attack_resilience/live_collusion_suite.py` điều phối
B1–B9, 180 s mỗi case (B8: ≥ 10 phút; B9: 4 giá trị b × 2 mục tiêu), 3 trial.

**13c. Replay checker ngữ nghĩa**

Mới: `engramd debug replay-fsm --rpc <url> --from 1 --to <h> --honest node01,…,node03,node06,node07`:
- `--honest` là tập validator trung thực của từng case (ví dụ trên cho B1–B8 với node04, node05
  Byzantine); trong B9 tập này thay đổi theo b.
- Nạp key validator từ genesis vào keeper trong bộ nhớ (`colltest.MockStore`, như trong test).
- Mỗi height: lấy `/block`, giải mã `Txs[0]`, chạy `AggregateHealthCertificates` +
  `CalculateNextState` trên state đã replay, rồi áp `CommitFSMTransition`.
- Báo vi phạm: `FsmState` đã commit ≠ giá trị tính lại; aggregate số nằm ngoài [min, max] của các
  certificate honest trong tập; có tx withdrawal khi SOVEREIGN/RECOVERING.
- Kết quả `replay_violations.csv`; paper báo số dòng mỗi case (kỳ vọng 0).

Trạng thái code:
- Đã làm 13a: `x/sovereignty/byzantine.go` (`false_observation`, `bias_certificate[:best]`,
  `omit_certs:<k>`, `stale_receipt`, `include_withdrawal`; ghép bằng `;`),
  `docker/engram-byzantine.yml` (biến `ENGRAM_BYZANTINE_BEHAVIOR_NODE0X` mỗi node, rỗng = trung thực)
  và dịch vụ double-sign thứ hai cho node05. Khác kế hoạch: dùng biến theo từng node thay cho
  `ENGRAM_BYZANTINE_NODES`; B8 không cần hook.
- Đã làm 13c: `x/sovereignty/replay.go` + `engramd debug replay-fsm` (`--honest`/`--byzantine`,
  `--report-from`, `--check-committed`). Chạy ngoài container validator (đã ~80% giới hạn 512 MiB),
  lấy `reanchoring_proof_valid` từ chuỗi vì proof ZK thật không tái hiện được từ byte của block.
  Kiểm tra trên 726 block có một lần SOVEREIGN→RECOVERING→ANCHORED thật: 0 vi phạm.
- Đã làm 13b ở mức in-process: `collusion_test.go` phủ B1–B3, B5, B6 và đối chứng âm B9 ở n=7
  (b ≤ 2 nằm trong khoảng honest; b = 3 với leader Byzantine thoát khỏi khoảng đó).
- Đã làm `scripts/e8_attack_resilience/live_collusion_suite.py` (B1–B9, eclipse thật, DA outage,
  double-sign hai node, replay mỗi case).
- Lỗi tìm thấy khi chạy live: `k.Params` chỉ gán trong `InitChainer`, nên validator khởi động lại
  dùng `DefaultParams()` thay vì Params genesis và lệch AppHash (panic `wrong Block.Header.AppHash`).
  Đã sửa bằng `EngramApp.LoadGenesisParams` (commit `cb7458b`); kết quả live trước bản sửa không dùng.
- Đang chạy (bắt đầu 2026-10-02 22:35, dự kiến xong khoảng 06:00 hôm sau): B1–B7 3 trial,
  B8 1 trial, B9 5 trial, 120 s tấn công mỗi case (kế hoạch ban đầu 180 s).
- Chưa làm: cập nhật `docs/EXPERIMENT.md` và paper bằng số liệu live (`evaluation.tex`,
  `experimental_setup.tex`, `system_design.tex`), hình B9, chạy lại A1/A2 ở n=7, mục 14.
- Giới hạn: B7/B8 chỉ đo, không khẳng định chuyển trạng thái; thông số n=1 (B8) và n=3/5 chỉ đủ
  báo số lần đạt, không báo độ lệch chuẩn.

### 14. Làm lại A3, A4, A7, A10 [M]

Các dòng này hiện chưa chứng minh được cơ chế phòng thủ đã kích hoạt.
- A3 (data withholding), A4 (forged BTC receipt): đếm `ProcessProposal REJECT reason=…` theo lý do
  trên các validator trung thực, chứng minh chính kiểm tra DA/BTC đã chặn.
- A7 (censorship): honest proposer đưa tx vào trước khi ngưỡng kích hoạt, nên cơ chế chưa được thử.
  Cho nhiều proposer cùng kiểm duyệt (node04 + node05, mục 13) hoặc giảm `MaxIgnoreRounds` trong lần
  chạy này, để `IsCensoring` thực sự bắn.
- A10 (timeout flood): ở 20 msg/s rate limiter không drop gì. Quét cường độ 20, 50, 100, 200,
  500 msg/s đến khi limiter bắt đầu drop; báo block rate theo cường độ, so với ngưỡng "cadence held"
  (mục 2).

---

## Giai đoạn 3 — Thay đổi giao thức

Thứ tự: 15 (nhỏ nhất) → 16 → 17. Mục 18–19 chỉ đụng `spec/`, làm song song.

### 15. State root trong checkpoint Bitcoin [T][B]

Payload OP_RETURN hiện là `"ENGR" ‖ height` (12 byte, `x/anchor/anchor.go:113-115`): Bitcoin chỉ
timestamp một con số, không gắn với trạng thái nào. Ô P_settle của mục 5 cần mục này.
1. Payload thành `"ENGR" ‖ height(8) ‖ root(32)` = 44 byte (< giới hạn 80 byte). Root = block hash
   CometBFT của `height` (lấy từ block store cục bộ / `ctx.BlockHeader()` của height kế). Sau khi một
   recovery proof được chấp nhận, checkpoint đầu tiên mang `LastAnchoredRoot`, gắn khoảng vừa phục
   hồi với dấu thời gian Bitcoin.
2. `BlockContainsTag` vẫn khớp theo tiền tố tag; `VerifyAnchor` so thêm root với block hash cục bộ
   của height đó — lệch thì tính là kiểm tra lại anchor thất bại.
3. Test trong `anchor_test.go` / `verify_test.go` (encode/decode, nhánh lệch); chạy lại E10 một lần
   để kiểm tra hồi quy.
4. Paper §3.8: bỏ `\pending`, ghi root nào được commit.

Trạng thái code:
- Đã làm: payload 44 byte (`anchor.EncodePayload`), `VerifyAnchor` so root với block hash CometBFT cục bộ
  (nguồn: `cometBlockHashSource` trong `cmd/engramd/comet_root.go`), unit test encode/decode và nhánh
  lệch. Checkpoint là height đã commit gần nhất (`BlockHeight()-1`), vì block đang đề xuất chưa có hash.
- Chưa làm: checkpoint đầu tiên sau recovery mang `LastAnchoredRoot` (đang commit block hash của
  height, chưa phải root Poseidon của chứng minh); chạy lại E10; sửa paper.
- Spec không đổi: spec trừu tượng hoá hash Bitcoin (`ExpectedBlockHash`), không mô hình payload.

### 16. Hysteresis thích nghi ANCHORED↔SUSPICIOUS [T]

Thay mặc định tĩnh (2, quá thấp dưới jitter WAN theo E5 live) bằng
`k_eff = min(k_max, k0 · 2^c)`: bộ đếm flap `c` tăng mỗi lần ANCHORED→SUSPICIOUS→ANCHORED, giảm sau
`W` block liên tiếp không flap.

Spec trước (`spec/core/EngramFSM.tla`):
- Biến mới `flap_count`, `flap_free_blocks`; hằng `MAX_ANCHORED_DOWN_HYST`, `FLAP_DECAY_WINDOW`.
- Operator mới `EffectiveAnchoredDownHysteresisThreshold` theo mẫu `EffectiveDownHysteresisThreshold`
  (dòng 84); dùng trong nhánh warning của ANCHORED trong `CalculateNextFSMState` (dòng 388+), không
  đổi thứ tự nhánh.
- Cập nhật bộ đếm trong bước FSM; invariant mới `FlapCountBounded`.
- Chạy lại `MC_FSMSafety` / `MC_FSMLiveness` và các ablation Apalache; cập nhật
  `tab:formal-verification`.

Go (quy tắc `go-spec-fidelity`):
- `keeper.go`: collection `FlapCount`, `FlapFreeBlocks` (prefix trống tiếp theo).
- `types/params.go`: `MaxAnchoredDownHysteresisThreshold`, `FlapDecayWindow`; kiểm tra
  `k_max ≥ DownHysteresisThreshold`.
- `keeper/circuit_breaker.go`: `EffectiveAnchoredDownHysteresisThreshold` cạnh
  `EffectiveDownHysteresisThreshold`; hàm thuần `NextFlapState(prev, next, count, freeBlocks, p)` theo
  mẫu `NextFailedRecoveryAttempts` (dòng 167).
- Nhánh ANCHORED của `CalculateNextState` dùng hàm mới; `FSMInput` thêm `FlapCount`;
  `CommitFSMTransition` lưu bộ đếm. Mọi đầu vào là committed state → tất định.
- Unit test: nhân đôi, chặn trên, giảm dần, không đổi khi transition không phải flap.

Thí nghiệm (`scripts/e5_hysteresis_flapping`, sweep live cạnh xuống): k=2 tĩnh, k=6 tĩnh, thích
nghi (k0=2, k_max=8, W=100), 3 trial mỗi cấu hình, hai chế độ: jitter WAN không lỗi (số flap,
uptime) và WAN + S2 BTC congestion (độ trễ phát hiện lẫn độ trễ phục hồi). Kỳ vọng: thích nghi bằng
k=6 về flap và bằng k=2 về độ trễ phát hiện. Cập nhật §6.3.

### 17. Dữ liệu DA thật + backfill khoảng Sovereign [T][B]

`sensors_refresh.go:52` hiện publish `da.HeightMarker(height)` — không có dữ liệu block — với tối
đa một lần gửi đang chờ; sau outage `verifiedHeight` nhảy thẳng lên height hiện tại
(`DANormalUpdate`, `EngramFSM.tla:196-212`), nên khoảng SOVEREIGN không bao giờ được publish. Ô
P_avail của mục 5 cần mục này.

**17a. Publish dữ liệu block thật, liên tục** (`x/da/publisher.go`)
- Nội dung blob: tx của block đã commit, publisher lấy ngoài consensus từ RPC CometBFT cục bộ
  (`/block?height=h`) — không bao giờ ghi vào committed state.
- Thay `pendingEngramHeight` đơn lẻ bằng hàng đợi: height tiếp theo = `contiguousHeight + 1`; gom các
  height liên tiếp tối đa ~1,5 MB mỗi blob; giữ quy tắc gửi lại `maxPendingBlocks` cho từng lô.
- Metric mới `contiguousHeight`: height `h` cao nhất mà mọi height ≤ `h` đã xác nhận lấy lại được.
  Khi outage, gửi thất bại và hàng đợi dài ra; khi phục hồi, hàng đợi rút theo thứ tự height.
- Kiểm tra `scripts/testnet_fund_celestia_bridge.sh` cấp đủ TIA cho payload thật.

**17b. Tách liveness đầu chuỗi và backfill**

Nếu dùng `contiguousHeight` làm `da_gap`, DA sẽ "không khoẻ" suốt quá trình backfill và đẩy
RECOVERING về SOVEREIGN. Vì vậy:
- `da_gap` vẫn đo liveness đầu chuỗi (lần gửi mới nhất có lên được không).
- Trường mới `PeripheralMetrics.da_contiguous_height` (proto, gom bằng weighted median trong
  `AggregateHealthCertificates`).
- Keeper ghi `RecoveringEntryHeight` khi SOVEREIGN→RECOVERING.
- Điều kiện RECOVERING→ANCHORED: `da_contiguous_height ≥ RecoveringEntryHeight - 1`.

**17c. Spec**
- `EngramFSM.tla`: biến `h_da_contiguous`, action `DABackfill` (tăng ≤ `h_engram_current`), biến
  `recovering_entry_height`; thêm điều kiện vào nhánh RECOVERING→ANCHORED của `CalculateNextFSMState`
  tại chỗ.
- Invariant `AnchoredImpliesSovereignDataPublished`; liveness cần weak fairness trên `DABackfill`.
- Ablation: bỏ điều kiện → Apalache tìm phản ví dụ (dòng mới trong `tab:ablation`).

**17d. Kiểm chứng**
- Unit: thứ tự hàng đợi, xử lý khoảng trống, tăng liên tục; điều kiện trong `CalculateNextState`.
- Live (S3 DA outage, 3 độ dài outage): thời gian ở RECOVERING theo độ dài khoảng Sovereign; sau đó
  `scripts/e3_failure_matrix/verify_backfill.py` lấy lại mọi height của khoảng từ Celestia
  (`blob.GetAll`) và so tx hash với block Engram (kỳ vọng khớp 100%).
- Paper: bỏ `\pending` ở §3.8, §5.4; bỏ bullet giới hạn "DA payload".

Trạng thái code:
- Đã làm 17a: `da.Publisher` publish block data thật theo lô, `ContiguousHeight()` tăng theo thứ tự;
  nguồn block là `cometBlockDataSource` (RPC CometBFT cục bộ).
- Đã làm 17b: `PeripheralMetrics.da_contiguous_height` (proto field 15, weighted median),
  `keeper.RecoveringEntryHeight`, `keeper.DABackfillValid`, điều kiện trong `CalculateNextState`.
- Đã làm 17c ở mức spec: biến Boolean `da_backfill_valid` thay cho `h_da_contiguous` +
  `recovering_entry_height` (giữ không gian trạng thái nhỏ), action `DABackfill`,
  `SovereignDataPublishedSafety`. `MC_FSMSafety` và `MC_FSMLiveness` đã chạy lại, 0 vi phạm; lớp Server chưa chạy lại.
- Đã làm 17d một phần: unit test publisher, gate, aggregation, e2e `StaysRecoveringUntilSovereignIntervalDataIsPublished`.
- Chưa làm: ablation bỏ điều kiện (dòng mới trong `tab:ablation`), chạy live S3 + `verify_backfill.py`,
  chạy lại TLC lớp Server, bỏ `\pending` trong paper.
- Khác kế hoạch: `da_gap` vẫn là head liveness (`VerifiedHeight` = height lúc lô được xác nhận), nên DA
  vẫn khoẻ trong lúc backfill; `da_contiguous_height` chỉ gate cạnh RECOVERING→ANCHORED.

### 18. Mở rộng mô hình TLA+ của health certificate [B]

Đã có (commit `1892bde`): quan sát Boolean của từng validator `sensor_view`
(`is_healthy`/`is_warning`/`is_critical`) do `ServerExtendVote` ghi (`EngramServer.tla`), Byzantine làm
giả `sensor_view`, proposer chọn bất kỳ tập ≥ `THRESHOLD2`, đa số theo từng trường
`AggregateHealthCertificate` (`EngramFSM.tla`), và `IsWellFormedHealthCertificate`
(`EngramTendermint.tla`). Các check này nằm ở lớp server nên hiện chỉ được phủ bởi các config
refinement chưa hội tụ (mục 19).

Còn thiếu so với implementation và Lemma 1:
- Trường số và weighted median (Go dùng weighted median cho `btc_gap`/`da_gap`); mô hình chỉ có
  Boolean.
- Validator trung thực bị eclipse: `sensor_view` honest luôn bằng predicate thật; chỉ Byzantine làm
  lệch.
- Chữ ký và ràng buộc height: ngoài phạm vi (spec không mô hình mật mã) — giữ nguyên.

Kế hoạch:
- Thêm quan sát số nhỏ (ví dụ `obs_gap[v] \in 0..3`) với operator trung vị cùng trọng số; invariant
  `AggWithinHonestRange`.
- Cho `sensor_view` honest lệch không tất định khi `~IsP2PQualityHealthy` ở node đó.
- Kiểm tra bằng Apalache trên một driver certificate độc lập (N=4), không phụ thuộc mục 19.
- Paper: đoạn "Health certificates in the model" ở §4 và bullet "Formal scope" ở §7.3.

### 19. Refinement C1 [T]

TLC nguyên khối trên `MC_StressC1Safety.cfg` không hội tụ; cần đổi cách chia bài toán. Các hướng, từ
rẻ đến đắt:
1. **Refinement hai bước** (đề xuất): (a) lớp Tendermint refines abstract consensus — kiểm tra riêng;
   (b) lớp server với Tendermint **thay bằng abstract consensus** refines spec đích — không còn
   prevote/precommit/round nên không gian trạng thái nhỏ hơn nhiều. Ghép bằng tính bắc cầu.
2. **Kiểm tra quy nạp bằng Apalache**, dùng cho bước (b) nếu TLC vẫn nặng:
   - `EngramServerRefinement_Ind.tla`: `IndInv` = type invariant ∧ invariant an toàn của server ∧ tính
     nhất quán của refinement mapping; biến ghost `prevAbs` giữ trạng thái abstract đã map.
   - Nghĩa vụ, mỗi cái `--length` 0 hoặc 1: `Init ⇒ IndInv`; `IndInv ∧ Next ⇒ IndInv'`; với mỗi
     disjunct `A_i` của `Next` server: `IndInv ∧ A_i ⇒ AbstractNext(prevAbs, abs) ∨ UNCHANGED abs`,
     kiểm tra như state invariant trên `prevAbs` sau một bước.
   - Tăng cường `IndInv` từ mỗi phản ví dụ quy nạp đến khi mọi nghĩa vụ qua.
3. **Assume-guarantee** (định lý composition Abadi–Lamport): chặt hơn, phải viết interface spec.
4. **TLAPS**: đúng cho N tuỳ ý nhưng tốn hàng tháng — không làm cho paper này.

Liveness refinement vẫn dùng TLC ở N=4 với `SYMMETRY SymmetryPerms` và giới hạn số round.

Điều kiện dừng: C1 (N=4, f=1) safety refinement được chứng minh. Nếu sau 2 tuần vẫn mở, giữ framing
"layered specification + bounded model checking" và thay `\pending` ở §4.1 bằng một câu khẳng định
trạng thái. Nếu xong, Target Property 7.1 trong `spec/README.md` được nâng thành kết quả có ghi rõ
cấu hình.

---

## Giai đoạn 4 — Thí nghiệm cuối trên code cuối

### 20. Chạy lặp các thí nghiệm live [M]

Chạy sau khi mục 15–17 xong, dùng công cụ đo của mục 11 và tiêu chí của mục 2.
- E2 S1–S7 × 5 bằng `scripts/e2_fault_injection/confidence_interval_recovery_time.py` (~2–2,5 giờ):
  mean ± CI 95% cho thời gian phục hồi S7, kèm 5 giá trị gốc; không báo p95 qua các lần lặp (n=5
  quá nhỏ). Percentile block interval trong từng run giữ nguyên.
- Hiệu ứng carry-over của S1–S7: mỗi lần lặp xáo trộn thứ tự S2–S6 (S1 đầu, S7 cuối), và thêm một
  lượt chạy S5 độc lập từ trạng thái sạch.
- E9 composite × 5, P4 × 3: báo cáo như trên.
- E10: quét reorg ở depth 6–14 để tìm điểm FSM bắt đầu phản ứng; mỗi depth ≥ 3 lần.

### 21. Chạy đa máy có WAN impairment [T][M]

Mục tiêu: kết quả chính (composite fault, so sánh ba hệ của mục 12) không chạy trên một máy.
- Hạ tầng theo quyết định 1b: 7 VM validator (2 vCPU / 4 GB) ở ≥ 3 vùng + 1 VM hạ tầng (Bitcoin
  regtest 2 node + miner loop, `celestia-app`). Mỗi VM validator có bitcoind và Celestia bridge
  riêng để góc nhìn ngoại vi độc lập.
- Genesis: `engramd testnet init-files --v 7` ở máy local, rồi sửa `persistent_peers` sang IP public
  (`init-files` nhận hostname prefix, không nhận địa chỉ từng node); chỉ mở 26656 giữa các VM.
- Deploy: `scripts/multihost/deploy.sh` — scp thư mục node, `docker compose up` mỗi VM một
  validator; quy tắc xoá dữ liệu trước khi deploy lại trong `CLAUDE.md` áp dụng cho từng VM.
- Script tiêm lỗi hiện gọi `docker exec`; thêm adapter trong `scripts/framework` (docker hoặc ssh)
  để `live_combined_trace.py` chạy nguyên. Lỗi: dừng bridge qua ssh, file tạm dừng BTC qua ssh,
  `tc` trên chính VM.
- WAN impairment: WAN thật cộng netem theo mức {cơ sở, +100 ms ±30, +1% loss, +2% loss}.
- Tránh bùng nổ số lượt (3 hệ × 3 lỗi × 4 mức × 3 lần = 108): ma trận đầy đủ chỉ cho composite
  fault; lỗi đơn chạy ở một mức WAN. 3 trial; báo cạnh số liệu single-host.

### 22. Demo verifier ZK bên ngoài (tuỳ chọn) [B]

Hiện thực hoá lập luận ở 6c: xuất verifier Solidity bằng `bb` (kiểm tra phiên bản `bb` đang dùng có
hỗ trợ xuất verifier cho UltraHonk không), chạy trên một EVM devnet, verify recovery proof thật, đo
gas. Nếu không làm, giữ 6c ở mức mô tả verifier mục tiêu.

**Cân nhắc thêm (tốn công, chưa quyết)** [M]:
- Baseline P2P mạnh hơn peer-count-only (một detector eclipse/Sybil đã công bố). Tối thiểu nói rõ
  peer-count-only là lower bound.
- Adversary tối ưu hoá: grid/random search trên cường độ, thời điểm, node mục tiêu để tối đa hoá khả
  năng né detector. Nếu không làm, bỏ claim "adaptive adversary" khỏi paper.
- Bitcoin congestion thật (nghẽn mempool/fee) thay vì dừng submit checkpoint.

---

## Giai đoạn 5 — Hoàn thiện paper

### 23. Tái cấu trúc Evaluation thành X1–X4 [T]

Giữ 5 contribution (thứ tự theo quyết định 1f); chỉ tái cấu trúc §6:

| Thí nghiệm | Chứng minh | Gộp từ |
|---|---|---|
| X1 – Liveness vs. safety | Engram vừa live vừa an toàn; baseline chỉ được một | composite, S1–S7, mục 12, mục 21 |
| X2 – Certificate dưới f=2 collusion | Lemma 1 đứng vững; replay 0 vi phạm | mục 13, P3/P4 |
| X3 – Recovery | proof + DA backfill + hysteresis thích nghi phục hồi đúng, không flap | E5, mục 16, mục 17 |
| X4 – Chi phí | overhead đến từ đâu (round attribution) | E7, mục 11 |

- Appendix: so sánh backend Plonky3, A1–A10, Monte Carlo detector P2P, reorg depth. Ablation formal
  giữ ở §4.
- Nếu đến mốc 1g mà mục 12 hoặc 13 chưa có số liệu, dùng phương án dự phòng ở mục 1.

### 24. Hoàn thiện trước khi nộp

- Đối chiếu mọi entry mới trong `docs/paper/references.bib` với DBLP. Trường chưa chắc:
  `vesely2022plumo` (danh sách tác giả), `tas2023bitcoinpos` (trang/DOI), `azouvi2022pikachu`
  (venue), `karakostas2021checkpointing`, `braithwaite2020tendermint`, `bano2021twins` (trang),
  `hayashibara2004phi` (trang).
- Kiểm tra lại các ô ✓/◐ của hệ thống khác trong `tab:comparison` theo bài gốc của từng hệ.
- Compile trên Overleaf (gói mới: `forest`); kiểm tra cây taxonomy và hình kiến trúc không tràn lề.
- Tên tác giả, affiliation (`main.tex`), URL artifact và license (`conclusion.tex`).
- Rà lại toàn bài theo checklist của skill `paper-writing` (mục X).
- `grep -rn pending docs/paper/sections/` trả về rỗng.

---

## Việc chờ quyết định và tồn đọng

### 25. Chờ quyết định: thí nghiệm Celestia light client (DAS) song song

Ở quy mô này, bridge cho mỗi validator (tải và kiểm tra toàn bộ block) mạnh hơn DAS sampling; DAS
chỉ có giá trị khi block quá lớn để tải về. Light client chỉ thêm tính thực tế (cơ chế ở quy mô
mainnet), không thêm bảo mật hay hiệu năng.
- Cách ít rủi ro: container `celestia-light-node0X` chạy cạnh bridge như một thí nghiệm riêng;
  `x/da/publisher.go` vẫn dùng bridge. Báo cáo: bridge = cơ chế production, light client = minh hoạ
  DAS.
- Cần đồng ý trước khi làm.

### 26. Chờ quyết định: validator dùng Bitcoin light client (SPV thật)

`IsBtcSpvFailed` (`x/anchor/anchor.go`) không phải SPV: `VerifyAnchor`/`BlockContainsTag` gọi
`getblock` verbosity=2 lên `bitcoind` full node của từng validator. Khi mọi validator đều chạy full
node, SPV thật chỉ đổi độ chặt lấy băng thông không cần thiết — việc nên làm là đổi tên (ví dụ
`IsAnchorReverificationFailed`). Paper đã dùng "anchor re-verification".

SPV thật chỉ đáng làm nếu validator bỏ full node:
1. Thay `RPCClient` bằng client BIP157/158 nhúng (ví dụ `btcsuite/neutrino`).
2. Giữ `bitcoin-node01`/`02` làm full node cho việc đào và test harness.
3. Kiểm tra tag thành "yêu cầu `gettxoutproof` cho `pendingTxid` đã biết" thay vì quét toàn block.
4. Phương pháp `invalidateblock` của E10 vẫn chạy ở lớp full node; cần xác nhận.
5. Lợi ích phụ: bỏ tranh chấp `-rpcworkqueue` của 7 validator trên một `bitcoind`.

Thay đổi kiến trúc; cần đồng ý trước khi làm.

### 27. Tồn đọng

- `reanchoring-prover` chạy trong container bị OOM quá 8192M kể cả với Docker Desktop VM 12 GB; dùng
  `watch_and_prove.sh` trên host (`make testnet-up PROVER=host`). Lịch sử trong comment của
  `docker/reanchoring-prover.yml`.
- `docs/FEEDBACK.md` giữ untracked/riêng tư — không commit.
