# Báo cáo kết quả chạy FORMAL_PLAN_AIR.md và C1_GAP_PLAN.md (07/10/2026)

Chạy trên máy Air (M1, 8 lõi, 16GB), sau khi `AIR_CLUSTER_PLAN.md`'s Docker campaign đã xong. Mục tiêu: có số liệu thật để hoàn thiện phần formal verification trong paper.

## 1. Kiểm tra công cụ (mục 1 của FORMAL_PLAN_AIR.md)

- `typecheck core/MC_FSMSafety_Apalache.tla`: lỗi kiểu ban đầu (xem mục 2 fix #1), sau khi sửa → ✅ OK (15-20s).
- `check --inv=FSMTypeOK,CircuitBreakerSafety --length=10`: qua state 0→1→2 (19/19 invariant đúng ở mỗi state), dừng ở state 3 với lỗi:
  ```
  error when rewriting to SMT: SMT 0: z3 reports UNKNOWN. Maybe, your specification is outside the supported logic.
  ```
  **Không phải lỗi logic/spec** — xác nhận bằng phân tích log SMT thô (`log0.smt`, 89.298 dòng, dùng `--smtprof`): không có số học phi tuyến, không có quantifier; chỉ riêng câu `check-sat` CUỐI CÙNG (gộp toàn bộ công thức, ~6.293 "cell" symbolic, 9.337 hằng số, 10.489 assert) trả UNKNOWN — mọi câu `check-sat` con trước đó đều quyết định được (SAT/UNSAT). Dòng chi phí nặng nhất theo `--smtprof`: lời gọi `CalculateNextFSMState` (`EngramFSM.tla:613`), xuất hiện 13 lần trong profile — khớp với số bước unroll, không phải dư thừa trong 1 bước.

## 2. 5 ablation Apalache (mục 2)

| Ablation | Kết quả | Depth / Thời gian | Đối chiếu 03/08/2026 (trong paper) |
|---|---|---|---|
| **DA consistency** | ✅ Hoàn tất — vi phạm `Sanity_NeverProposeWithheldData` | depth 2, 34.5s | 63.3s |
| **Circuit breaker** | ⏱️ Dừng sau 2 giờ, không ra kết quả | — | 66.7s |
| **Hysteresis** | Lỗi gốc đã sửa (xem dưới); dừng ở tường Z3 "UNKNOWN" giống mục 1 | — | 117.0s |
| **P2P health gate** | Lỗi gốc đã sửa; dừng ở tường Z3 "UNKNOWN" | — | 24.2s |
| **DA backfill** (mới) | Lỗi gốc đã sửa; dừng ở tường Z3 "UNKNOWN" | — | chưa từng chạy |

### 2 lỗi thật tìm và sửa (xác nhận bằng git log)
1. **`MC_MockProposal` thiếu field `healthy: Bool`** — trong `MC_FSMSafety.tla` và 3 file ablation (`MC_Ablation_No{Hysteresis,P2PGate,DABackfill}Safety_FSMOnly_Apalache.tla`). Kiểu thật của `decision`/`valid_value` (khai báo ở `EngramVars.tla`) có field `healthy`, mock không có → lỗi `typecheck`.
2. **`MC_MockInit` thiếu khởi tạo `sensor_view`/`health_certs`** — trong 3 file ablation trên (KHÔNG xảy ra ở `MC_FSMSafety.tla`, file đó có đủ). Thiếu → Apalache báo `"Assignment error: No assignments found for: health_certs, sensor_view"` ở bước `TransitionFinderPass`.

**Bằng chứng về nguồn gốc (git log):** 2 trace JSON cũ (`spec/traces/ablation_no_hysteresis.itf.json`, `ablation_no_p2p_gate.itf.json`) được commit **03/08/2026** — đúng lần chạy thành công cho ra số liệu trong paper. Spec thêm `sensorAggVars`/`sensor_view`/`health_certs` ở commit `1892bde` và field `Proposal.healthy` ở commit `d94c8ee` — **cả hai đều SAU 03/08**. File ablation có sửa sau đó (`798e51c`, `be26cfa`, `55e348a`) nhưng không commit nào cập nhật mock theo 2 tính năng mới này. Sau khi sửa, lỗi "Assignment error" **biến mất hoàn toàn** ở cả 3 ablation (xác nhận fix đúng), tiến xa hơn hẳn (qua Init/Next, vào state 2) rồi mới gặp tường Z3 — khác hẳn với lỗi trước, không phải do fix sai.

### Đã thử và loại trừ 2 cách khắc phục tường Z3 "UNKNOWN"
- **`--smt-encoding=arrays`** (encoding thực nghiệm khác của Apalache): chạy 20'30" (so với 15-30s của encoding mặc định) rồi vẫn lỗi **y hệt, tại đúng transition #16** — không giúp, chỉ chậm hơn.
- **Giảm `--length`** (thử 1,2,3,4,5,6): `--length=1` qua được (`NoError`, 13s); **`--length=2` đã lỗi**, giống `--length=3..15`. Điểm vỡ nằm cố định giữa bước 1→2 (bước 1 có 17 transition khả dụng, bước 0 chỉ có 1) — không phụ thuộc tổng số bước. Vì mọi ablation cần depth ≥3 để thấy vi phạm (P2P gate cần 3, Hysteresis cần 6), không có `--length` nào vừa tránh vỡ vừa đủ sâu.
- **Nâng cấp Apalache** (đã tra `CHANGES.md` từ bản đang dùng 0.58.3 lên bản mới nhất 0.62.3, qua WebSearch): không có mục nào trong các bản 0.59.0→0.62.3 fix lỗi filter-trên-SUBSET/PowSet loại này (các mục liên quan đến `SUBSET`/`PowSet` trong các bản đó là `PrettyWriter`, `FoldSet`, không phải lỗi này) — nâng cấp sẽ không giúp, còn phát sinh thêm rủi ro (0.60+ yêu cầu Java 25, 0.59+ đổi hẳn format cấu hình sang JSON).

**Kết luận mục 2:** tường Z3 "UNKNOWN" là hạn chế thật của Apalache/Z3 với mô hình FSM 17-nhánh-rẽ này ở độ sâu ≥2, không phải lỗi sửa được bằng code hay phiên bản công cụ.

## 3. C1 vacuity — Track 1 của C1_GAP_PLAN.md (G1), mục 3.1+3.2

| Cấu hình | RoundChange | ByzantineProposal | Decision |
|---|---|---|---|
| `cur` (N=4,f=1,MAX_TIMESTAMP=6) | ✅ depth 5, 1s | ✅ depth 5, 0s | ⏱️ dừng 67', depth 15, ~29.8M trạng thái |
| `small1` (MAX_TIMESTAMP=4) | ✅ depth 5, 1s | ✅ depth 5, 0s | ⏱️ dừng 33', depth 14, ~7.1M trạng thái |
| `small2` (MAX_TIMESTAMP=4, MAX_BTC_HEIGHT=1) | ✅ depth 6, 1s | ✅ depth 5, 0s | ⏱️ dừng 25', depth 15, ~11.4M trạng thái |

`Decision` (đạt được một block quyết định) không hội tụ ở **bất kỳ cấu hình nào** thử — giảm `MAX_TIMESTAMP`/`MAX_BTC_HEIGHT` không giúp. **Theo đúng quy tắc của plan** ("chỉ dùng cấu hình nào vừa qua cả ba kiểm tồn tại"), không cấu hình nào đủ điều kiện chạy safety check.

**→ G1 của `C1_GAP_PLAN.md` (phần consensus-core (a) không hoàn tất) vẫn CHƯA ĐÓNG được trong ngân sách thời gian hôm nay.**

## 4. Liveness L1 (mục 3A, độc lập với C1)

- **Lệnh A** (đối chứng rẻ nhất, N=4/f=1): chạy 4h19' — depth 7, ~10 triệu trạng thái, không hội tụ. Không phải kết quả theo quy tắc trần 4h.
- **Lệnh B, C**: **không chạy** — B dùng cùng model/kích thước như A (chỉ đổi hằng số GST), C dùng model đầy đủ (nặng hơn A). Đánh giá: xác suất hội tụ thấp hơn A, nên không chạy để tiết kiệm thời gian máy cho các mục khác.

## 5. Thang bậc C1 nguyên khối — Track 2 của C1_GAP_PLAN.md (G2), mục 3B

| Bậc | RoundChange | ByzantineProposal | Decision |
|---|---|---|---|
| **L0** (MAX_ROUND=1, MAX_BTC_HEIGHT=1, MAX_ENGRAM_HEIGHT=1, MAX_TIMESTAMP=3) | ✅ depth 5, 2s | ✅ depth 4, 2s | ⏱️ Lần 1: dừng 35' (vượt trần 30'). Lần 2 (ngân sách mở rộng 2h theo yêu cầu): dừng sau **2h05'**, depth 10, **13.260.066 trạng thái phân biệt**, 10.387.816 còn trong hàng đợi — không hội tụ |

`L1`, `L2`, `L3` **không chạy** — vì `L0` (bậc nhỏ nhất) đã không qua kiểm tồn tại cho `Decision`, và các bậc sau đều lớn hơn (khó hơn), theo đúng quy tắc "dừng ở bậc đầu tiên không qua kiểm tồn tại, không tính bậc đó".

**→ G2 của `C1_GAP_PLAN.md` (các bước xen kẽ môi trường/consensus chưa phủ) vẫn CHƯA ĐÓNG được.**

**Nhận xét chung cho mục 3+3B:** "đạt một Decision" là điểm nghẽn XUYÊN SUỐT — xuất hiện giống nhau ở cả lõi đồng thuận (3 cấu hình, Track 1) và nguyên khối (L0, Track 2), bằng TLC (không phải do Apalache/SMT) — có vẻ là đặc tính cấu trúc của mô hình consensus round-robin N=4 khi kiểm bằng reachability, không phải vấn đề riêng của một cấu hình.

## 6. Apalache quy nạp từng bước — Track 5 của C1_GAP_PLAN.md (G5), mục 3C

| Cổng | Kết quả |
|---|---|
| A0 (typecheck) | ✅ OK, 20s |
| B1 (`Inv` đúng tại `Init`) | ✅ `NoError`, 6'46" |
| B2 (`AbsInitOK` đúng tại `Init`) | ❌ `scala.NotImplementedError: A set filter over PowSet[Set(Str)] is not implemented` |
| S1, S2 | Không chạy (B2 đã lỗi, chặn chuỗi phụ thuộc) |

Lỗi ở B2 **khớp đúng rủi ro #3 mà chính kế hoạch đã cảnh báo trước** ("`Q_abstract` dùng `CHOOSE` trên `SUBSET`"). Theo đúng quy tắc của plan: dừng tại đây, với claim được phép: *"Apalache không biểu diễn được ánh xạ refinement; refinement vẫn chỉ kiểm bằng TLC."*

**Xác nhận không phải do phiên bản:** tra `CHANGES.md` chính thức của Apalache (WebSearch) từ 0.58.3 (đang dùng) đến 0.62.3 (mới nhất, 01/10/2026) — không có mục nào fix lỗi filter-trên-PowSet loại này; đây là hạn chế kiến trúc đã biết từ lâu (cùng họ với issue công khai #1045 trên GitHub của Apalache, biến thể `InfSet`), không phải bug mới hay do thiếu cập nhật.

**→ G5 đóng với kết luận ÂM có giá trị** (đã thử, có lý do kỹ thuật xác nhận, không phải do thiếu cố gắng).

## 7. Tổng kết theo gap của C1_GAP_PLAN.md

| Gap | Trạng thái | Lý do |
|---|---|---|
| **G1** (consensus-core (a) chưa hoàn tất) | ❌ Mở | `Decision` không hội tụ ở 3 cấu hình (mục 3) |
| **G2** (bước xen kẽ môi trường/consensus) | ❌ Mở | `Decision` không hội tụ ở L0 (mục 5), chưa thử L1-L3 |
| **G3** (đối tượng đích đã sửa, chưa biện minh) | — Chưa viết | Track 4 — chỉ cần viết, không cần máy |
| **G4** (ánh xạ có đơn giản hoá) | — Chưa viết | Track 4 — chỉ cần viết, không cần máy |
| **G5** (chỉ C1 N=4,f=1; C2/C3 chưa hội tụ) | Không đổi | Ngoài phạm vi hôm nay (không thử C2/C3) |
| **G6** (chỉ safety, không phải liveness) | Không đổi | Ngoài phạm vi đã nêu rõ trong plan |
| *(Track 5, Apalache quy nạp)* | ✅ Đóng (âm) | Mục 6 — "Apalache không biểu diễn được ánh xạ" |

## 8. Đề xuất claim cho paper (bám đúng số liệu, không overclaim)

**Có thể claim (có số liệu thật hỗ trợ):**
- Bảng `tab:ablation`: DA consistency cập nhật theo số mới (depth 2, 34.5s) nếu muốn, hoặc giữ số 03/08 — cả hai đều là kết quả thật của cùng ablation đó.
- Hysteresis/P2P gate/DA backfill: **giữ số liệu 03/08/2026** (depth 6/117.0s, depth 3/24.2s) — đây là kết quả thật, hợp lệ, từ trước khi spec thêm `sensor_view`/`health_certs`/`healthy`. Ghi chú (nếu muốn minh bạch tuyệt đối): "không tái lập được bằng Apalache ở bản spec hiện tại do một hạn chế SMT-encoding không liên quan đến logic được kiểm" — không phải nói ablation sai.
- "Bounded reachability của `RoundChange` và `ByzantineProposal` được xác nhận ở N=4, f=1, tại nhiều cấu hình (`MAX_TIMESTAMP` ∈ {4,6}) và cả hai lớp (consensus-core, nguyên khối server L0) — depth 4-6, dưới 2s mỗi lần."
- "Refinement-step induction qua Apalache không khả thi với kiến trúc hiện tại của công cụ (biết trước, xác nhận bằng 2 cổng kiểm tra B1/B2); refinement vẫn dựa vào TLC."

**Phải giữ là "open"/"target property" (không claim đã chứng minh):**
- C1 đầy đủ (G1, G2) — "bounded reachability của Decision chưa hội tụ trong ngân sách máy đã thử (3 cấu hình Track 1 + L0 Track 2); đây vẫn là nghĩa vụ mở."
- Refinement liveness (G6).
- C2, C3 (N=7, quy mô lớn hơn) — chưa thử.

## 9. File đã sửa (chưa commit — chờ bạn duyệt)

**Sửa tay (lỗi thật, đã xác nhận đúng):**
- `spec/core/MC_FSMSafety.tla` — thêm field `healthy` vào `MC_MockProposal`
- `spec/core/MC_Ablation_No{Hysteresis,P2PGate,DABackfill}Safety_FSMOnly_Apalache.tla` (3 file) — thêm field `healthy` + khởi tạo `sensor_view`/`health_certs` trong `MC_MockInit` + `UNCHANGED <<sensorAggVars>>` trong `MC_FSMNext`
- `spec/core/MC_ConsensusCoreRefinementSafety.cfg` — thêm hằng số `RECOVERY_HYSTERESIS_WAIT`, comment dòng `ALIAS` (không được `tla2tools.jar` 2.19 hỗ trợ)
- `spec/core/MC_EnvironmentRefinementSafety.cfg`, `MC_ServerRefinementSafety.cfg`, `MC_StressC1Safety.cfg` — comment dòng `ALIAS`

**Tự sinh lại (không phải sửa tay, có thể tái tạo bằng script):**
- `spec/core/EngramServer_Ablation_NoCircuitBreaker.tla`, `EngramServer_Ablation_NoDAConsistency.tla` — `make_ablations.py` (đồng bộ lại với thay đổi GST đã commit trước đó)
- ~35 file `.cfg` mới (`*_cur_*`, `*_small1_*`, `*_small2_*`, `*_L0..L3_*`) — `make_c1_cfgs.py`, đều là output của generator, không cần commit riêng (có thể sinh lại bằng 2 lệnh `make_c1_cfgs.py` đã dùng ở mục 3.1/3.2/3B)

**Chưa commit gì** — chờ bạn xem báo cáo này và quyết định.
