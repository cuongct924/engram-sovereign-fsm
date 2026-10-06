# Kế hoạch chạy formal trên máy Mac Air M1 (16 GB, 8 lõi, SSD 256 GB)

Mục tiêu: có số liệu formal cuối cùng để cập nhật Section 5, `spec/README.md` và báo cáo, mà không chạm vào cụm Docker ở máy chính.
Không chạy TLC/Apalache trên máy chính khi cụm đang đo.

## Thứ tự và trần thời gian (làm theo đúng thứ tự; mỗi lần chỉ một công cụ, không chạy TLC song song với Apalache)
| # | Mục | Việc | Trần |
|---|---|---|---|
| 1 | 0–1 | Cài và kiểm tra công cụ | 40 phút |
| 2 | 2 | Năm ablation Apalache (gồm lần chạy lại sau đổi tên `Sanity_NeverAttemptWithdrawalWhileLocked`) | 2 giờ |
| 3 | 3A | Liveness L1 sau khi sửa GST | 3 lệnh × 4 giờ |
| 4 | 3 | C1 lõi đồng thuận (Track 1) | 6 giờ, qua đêm |
| 5 | 3B | C1 nguyên khối, thang cận L0–L3 (Track 2; thứ thầy hỏi) | 24 giờ tổng |
| 6 | 3C | Apalache kiểm tra bước refinement theo quy nạp | 1 ngày, chỉ khi 3B đạt L2 trở lên hoặc bạn muốn thử sớm |
Tổng trần khoảng 3 ngày máy. Hết trần thì dừng và gửi kết quả đang có; không kéo dài. Mọi chỗ "gửi lại" ở các mục là để Claude cập nhật paper và sửa spec; đừng tự sửa spec trên Air.

## 0. Chuẩn bị (khoảng 30 phút)
```bash
git clone <repo> engram-sovereign-fsm && cd engram-sovereign-fsm && git checkout main && git pull
brew install openjdk@17            # JDK 17+ bản arm64 (Apalache không chạy trên JDK 11)
export JAVA_HOME="$(/usr/libexec/java_home -v 17)"; export PATH="$JAVA_HOME/bin:$PATH"
mkdir -p ~/tools/apalache && cd ~/tools/apalache
curl -sL -o apalache.tgz https://github.com/apalache-mc/apalache/releases/download/v0.58.3/apalache.tgz && tar xzf apalache.tgz
mkdir -p ~/tools/tla && curl -sL -o ~/tools/tla/tla2tools.jar https://github.com/tlaplus/tlaplus/releases/latest/download/tla2tools.jar
cd ~/engram-sovereign-fsm && python3 spec/tools/make_ablations.py && git status --short spec   # phải không có thay đổi
```
Ghi lại phiên bản (`java -version`, `apalache-mc version`, TLC in ra khi chạy) vào báo cáo. Cắm điện, tắt chế độ ngủ. Còn trống ít nhất 40 GB đĩa (TLC ghi trạng thái ra đĩa).

Quy ước: `A=~/tools/apalache/apalache/bin/apalache-mc`, `TLC="java -Xmx11g -XX:+UseParallelGC -cp ~/tools/tla/tla2tools.jar tlc2.TLC"`, mọi lệnh chạy trong `spec/`.

## 1. Kiểm tra công cụ (5 phút)
```bash
cd spec
$A typecheck core/MC_FSMSafety_Apalache.tla
$A check --cinit=ApalacheCInit --init=MC_FSMInit --next=MC_FSMNext --inv=FSMTypeOK,CircuitBreakerSafety --length=10 core/MC_FSMSafety_Apalache.tla
```
Kỳ vọng: `NoError`. Nếu lỗi thì dừng và gửi nguyên văn lỗi.

## 2. Năm ablation bằng Apalache (khoảng 1–2 giờ)
Các module ablation nay được **sinh lại từ spec hiện tại** bằng `spec/tools/make_ablations.py` (các bản cũ là bản sao đã lệch spec). Mỗi lệnh: chạy `typecheck` trước, rồi `check`. Mỗi ablation **phải có vi phạm** (đó là mục đích). Ghi độ sâu, thời gian, lưu vết `.itf.json`.

| Ablation | Driver | Lệnh `check` (thêm `core/<driver>.tla` cuối) |
|---|---|---|
| Circuit breaker | `MC_Ablation_NoCircuitBreakerSafety_Apalache` | `--cinit=ApalacheCInit --init=MC_ServerInit --next=MC_ServerNext --inv=Sanity_NeverAttemptWithdrawalWhileLocked --length=12` |
| Hysteresis | `MC_Ablation_NoHysteresisSafety_FSMOnly_Apalache` | `--cinit=ApalacheCInit --init=MC_FSMInit --next=MC_FSMNext --inv=Sanity_NoIllegalHysteresisExit --length=15` |
| P2P gate | `MC_Ablation_NoP2PGateSafety_FSMOnly_Apalache` | `--cinit=ApalacheCInit --init=MC_FSMInit --next=MC_FSMNext --inv=Sanity_NoIllegalP2PRecovery --length=15` |
| DA consistency | `MC_Ablation_NoDAConsistencySafety_Apalache` | `--cinit=ApalacheCInit --init=MC_ServerInit --next=MC_ServerNext --inv=Sanity_NeverProposeWithheldData --length=12` |
| **DA backfill (mới, thay ablation f+1)** | `MC_Ablation_NoDABackfillSafety_FSMOnly_Apalache` | `--cinit=ApalacheCInit --init=MC_FSMInit --next=MC_FSMNext --inv=Sanity_NoIllegalBackfillExit --length=15` |

Kết quả cũ (trên bản sao cũ) để đối chiếu: độ sâu 2, 6, 3, 2 và 66,7 s, 117,0 s, 24,2 s, 63,3 s. Khác đi không sao, nhưng phải ghi số mới.
Nếu `typecheck` hoặc `check` lỗi cú pháp/kiểu (backfill chưa từng chạy, các module sinh lại chưa từng chạy), **dừng ở ablation đó**, lưu nguyên lỗi, làm tiếp ablation khác, gửi lại cho Claude sửa.
Đối chứng (không cần chạy lại): tính chất không bị ablation, `SovereignDataPublishedSafety` và các bất biến FSM, đã có kết quả TLC không vi phạm trong `MC_FSMSafety`.

## 3. C1 consensus-core (khoảng 4–6 giờ, chạy qua đêm)
Lưu ý: ở phần (a) môi trường bị đóng băng nên **FSM không chuyển trạng thái**; các chuyển trạng thái thuộc phần (b) (đã hoàn tất). Tiêu chí "đủ ý nghĩa" cho cấu hình nhỏ hơn vì vậy là ba hành vi của lõi đồng thuận: **đổi vòng, proposer Byzantine có đề xuất, có block được quyết định**.

3.1 Kiểm tra tồn tại hành vi trên cấu hình hiện tại (mỗi lệnh phải kết thúc bằng vi phạm trong vài phút):
```bash
cd spec && python3 tools/make_c1_cfgs.py --tag cur
for p in RoundChange ByzantineProposal Decision; do
  $TLC -workers 6 -config core/MC_ConsensusCoreVacuity_cur_$p.cfg core/MC_ConsensusCoreVacuity.tla 2>&1 | tail -15 > /tmp/vac_cur_$p.txt; done
```
(Cấu hình hiện tại lấy từ cfg gốc: N=4, f=1, MAX_ROUND=2, độ cao Bitcoin và Engram tối đa 2, MAX_TIMESTAMP=6.)

3.2 Hai cấu hình nhỏ hơn, mỗi cái kiểm tra tồn tại trước rồi mới chạy safety:
```bash
python3 tools/make_c1_cfgs.py --tag small1 --set MAX_TIMESTAMP=4
python3 tools/make_c1_cfgs.py --tag small2 --set MAX_TIMESTAMP=4 MAX_BTC_HEIGHT=1
# với mỗi tag: 3 lệnh vacuity như trên (phải vi phạm), sau đó:
$TLC -workers 6 -metadir /tmp/tlc_small1 -config core/MC_ConsensusCoreRefinementSafety_small1.cfg core/MC_ConsensusCoreRefinementSafety.tla | tee /tmp/c1_small1.log
```
Quy tắc: chỉ dùng cấu hình nào **vừa qua cả ba kiểm tra tồn tại, vừa chạy hoàn tất** (dòng "Model checking completed. No error has been found."). Giới hạn 4 giờ mỗi cấu hình; không xong thì dừng, ghi rõ "dừng" và số trạng thái, không gọi là kết quả. Ghi: số trạng thái sinh ra, số phân biệt, độ sâu, thời gian.

3.3 Quy nạp bằng Apalache cho C1: **không có file khung** (`EngramServerRefinement_Ind.tla` chưa tồn tại). Bỏ qua nếu hết thời gian; đây là việc viết mới (một ngày).

## 3A. Kiểm tra lại liveness L1 sau khi sửa mô hình GST (khoảng 2–6 giờ; chưa có kết quả nào)

**Đã đổi (chưa chạy TLC; chỉ phân tích cú pháp bằng SANY cho `EngramServer`, `MC_ServerRefinementLiveness`, `MC_ControlRun_FastForwardLiveness`):**
- `spec/core/EngramServer.tla`: bỏ `GSTReached` (vị từ tái diễn, gồm cả `state = "ANCHORED"` và số peer). Thay bằng `GSTTime == MAX_TIMESTAMP \div 3` và `AfterGST == real_time >= GSTTime` (đơn điệu vì `real_time` không giảm); `EventualDecisionUnderGSTLiveness == (<>AfterGST) ~> (\E p \in HonestNodes : step[p] = "DECIDED")`.
- `spec/core/MC_ServerRefinementLiveness.tla`: `Sanity_NeverReachGST == [] ~AfterGST`.
- Giữ nguyên: fairness (`MC_ServerFairness`, không điều kiện, vì weak fairness chỉ ràng buộc hành vi vô hạn nên độ trễ hữu hạn trước GST vẫn được phép), các `.cfg`, các thuộc tính khác (L5–L7, `ServerEventualDecisionLiveness`). Các bản ablation `NoFastForward` là bản sao cũ, không sinh lại, vẫn định nghĩa `GSTReached` cũ (ablation đó không có kết quả tin cậy).
- Điều cần cảnh giác khi đọc kết quả: trong mô hình `ServerAdvanceRealTime` bị tắt khi đã có QC ở vòng cao nhất, nên thời gian có thể đứng yên dưới `GSTTime`. Hành vi đó không thoả vế trái `<>AfterGST` và không phải phản ví dụ.

Lệnh (trong `spec/`, `TLC` như mục 0). Chạy lần lượt, ghi lại số trạng thái và thời gian; mỗi lệnh tối đa 4 giờ, không xong thì dừng và ghi "dừng".
```bash
cd spec
# 0) phân tích cú pháp (1 phút); phải không có lỗi
for m in EngramServer MC_ServerRefinementLiveness MC_ControlRun_FastForwardLiveness; do (cd core && java -cp ~/tools/tla/tla2tools.jar tla2sany.SANY $m.tla 2>&1 | tail -3); done

# A) đối chứng rẻ nhất: EngramServer thật, chỉ thuộc tính L1 (N=4, f=1)
$TLC -workers 6 -deadlock -metadir /tmp/tlc_l1_control -config core/MC_ControlRun_FastForwardLiveness.cfg core/MC_ControlRun_FastForwardLiveness.tla | tee /tmp/l1_control.log

# B) hai thời điểm GST khác (bản sao thư mục, không sửa repo)
for g in 1 "MAX_TIMESTAMP - 3"; do
  d=/tmp/core_gst_$(echo $g | tr -dc '0-9'); rm -rf $d; cp -r core $d
  sed -i '' "s/^GSTTime == .*/GSTTime == $g/" $d/EngramServer.tla
  $TLC -workers 6 -deadlock -metadir /tmp/tlc_l1_$(basename $d) -config $d/MC_ControlRun_FastForwardLiveness.cfg $d/MC_ControlRun_FastForwardLiveness.tla | tee /tmp/l1_$(basename $d).log
done

# C) mô hình đầy đủ nhưng chỉ L1 và ServerEventualDecisionLiveness
awk '/^PROPERTIES/ {print; print "    EventualDecisionUnderGSTLiveness"; print "    ServerEventualDecisionLiveness"; skip=1; next} !skip' core/MC_ServerRefinementLiveness.cfg > core/MC_ServerRefinementLiveness_L1.cfg
$TLC -workers 6 -deadlock -metadir /tmp/tlc_l1_full -config core/MC_ServerRefinementLiveness_L1.cfg core/MC_ServerRefinementLiveness.tla | tee /tmp/l1_full.log
rm core/MC_ServerRefinementLiveness_L1.cfg    # không commit
```
Nếu TLC báo `Deadlock reached`, kiểm tra trạng thái cuối: khi mọi validator trung thực ở `DECIDED` thì đó là kết thúc dự kiến (spec nói rõ có thể bế tắc khi tất cả đã quyết) và `-deadlock` bỏ qua nó.

**Cách đọc kết quả:**
| Kết quả | Ý nghĩa | Làm gì |
|---|---|---|
| `No error has been found` ở A, B, C | L1 đúng, ở mọi trạng thái FSM, trong cấu hình nhỏ, với ba thời điểm GST | Cập nhật `spec/README.md` (bảng trạng thái, L1: Checked, kèm cấu hình, số trạng thái, thời gian) và đoạn liveness của mục 5 |
| Vi phạm có vết | Có hành vi đạt `AfterGST` mà không ai quyết định. Xem trạng thái FSM tại điểm kẹt (`state`, receipts) và các hành động đang bật | Lưu vết; có thể là khoảng trống liveness thật (ví dụ ANCHORED chờ biên lai hỏng trước khi FSM chuyển) hoặc thiếu fairness cho FSM. Gửi lại cho Claude, không tự sửa |
| TLC báo lỗi về công thức thời gian | Cách viết `(<>AfterGST) ~> ...` không được chấp nhận | Gửi nguyên văn lỗi |
| Không xong sau 4 giờ | Không kết luận | Ghi "dừng", số trạng thái và độ sâu, như quy tắc ở mục 3.2 |

Gửi lại: ba file log, phiên bản TLC (in ra khi chạy), cấu hình và vết nếu có.

## 3B. C1 nguyên khối: thang cận cho `MC_ServerRefinementSafety` (trần 24 giờ)

Đây là Track 2 của `docs/C1_GAP_PLAN.md` và là thứ thầy hỏi: một cấu hình nhỏ hơn **vẫn có Byzantine proposer, đổi vòng, một chuyển trạng thái FSM và một block được quyết định**, chạy hoàn tất. Chỉ chạy nguyên khối (mọi hành động, môi trường và consensus cùng lúc) mới có chuyển trạng thái FSM; phần (a) ở mục 3 đóng băng môi trường nên không có. Kết quả chỉ được gọi là **bounded refinement sanity check**, không phải chứng minh C1.

Công cụ đã có trong repo (không cần viết thêm): `core/MC_ServerRefinementVacuity.tla` (năm điều kiện tồn tại `NotReached_*`) và `tools/make_c1_cfgs.py --layer server`.

```bash
cd spec
python3 tools/make_c1_cfgs.py --layer server --ladder     # ghi MC_ServerRefinementSafety_L{0..3}.cfg và MC_ServerRefinementVacuity_L{0..3}_<predicate>.cfg
git status --short core                                     # chỉ có các cfg mới, không commit chúng
```
Các bậc (mỗi bậc đổi thêm một cận so với bậc trước; cfg gốc là `MAX_ROUND=2, MAX_BTC_HEIGHT=3, MAX_ENGRAM_HEIGHT=3, MAX_TIMESTAMP=20`):
| Bậc | MAX_ROUND | MAX_BTC_HEIGHT | MAX_ENGRAM_HEIGHT | MAX_TIMESTAMP |
|---|---|---|---|---|
| L0 | 1 | 1 | 1 | 3 |
| L1 | 1 | 1 | 2 | 3 |
| L2 | 1 | 1 | 2 | 4 |
| L3 | 2 | 1 | 2 | 4 |

Với **mỗi bậc, lần lượt từ L0** (đặt `k=L0`, rồi `L1`, ...):

1. Kiểm tra tồn tại: năm lệnh, mỗi lệnh **phải kết thúc bằng vi phạm** (TLC in `Invariant NotReached_... is violated`). Tối đa 30 phút mỗi lệnh.
```bash
k=L0
for p in RoundChange ByzantineProposal Decision FSMTransition DecisionAfterFSMTransition; do
  $TLC -workers 6 -metadir /tmp/tlc_vac_${k}_$p -config core/MC_ServerRefinementVacuity_${k}_$p.cfg core/MC_ServerRefinementVacuity.tla 2>&1 | tail -25 > /tmp/vac_${k}_$p.txt
done
grep -l "is violated" /tmp/vac_${k}_*.txt | wc -l      # phải bằng 5
```
2. Chỉ khi cả năm đều vi phạm mới chạy safety (có `RefinementSafety` trong PROPERTIES). Tối đa 6 giờ mỗi bậc:
```bash
$TLC -workers 6 -metadir /tmp/tlc_mono_${k} -config core/MC_ServerRefinementSafety_${k}.cfg core/MC_ServerRefinementSafety.tla | tee /tmp/c1_mono_${k}.log
tail -8 /tmp/c1_mono_${k}.log
```
Hoàn tất là dòng `Model checking completed. No error has been found.`; ghi số trạng thái sinh ra và phân biệt, độ sâu và thời gian.

**Quy tắc dừng:** dừng ở bậc đầu tiên mà (i) có điều kiện tồn tại không vi phạm trong 30 phút, hoặc (ii) safety không hoàn tất sau 6 giờ, hoặc (iii) tổng thời gian của mục này chạm 24 giờ. Ghi "bậc lớn nhất hoàn tất" là bậc cuối cùng qua cả hai bước. Bậc không qua kiểm tra tồn tại **không được tính**, kể cả khi safety không báo lỗi.

**Cách đọc:**
| Kết quả | Claim được phép trong paper |
|---|---|
| Hoàn tất ít nhất một bậc | "Bounded refinement sanity check of the composed server layer at N=4, f=1, MAX_ROUND=r, MAX_ENGRAM_HEIGHT=h, MAX_TIMESTAMP=t, with a Byzantine proposer, a round change, an FSM transition and a block decision; this is not a proof of C1." Kèm số trạng thái và thời gian |
| Vi phạm `RefinementSafety` hoặc một bất biến | Lưu vết và gửi lại; có thể là lỗi ánh xạ thật. Không sửa trên Air |
| Không bậc nào hoàn tất | Giữ "cross-layer refinement remains open"; ghi bậc L0 cần bao nhiêu giờ và dừng ở đâu |

Gửi lại: các file `/tmp/vac_*.txt` và `/tmp/c1_mono_*.log` (hoặc ít nhất 40 dòng cuối mỗi file), phiên bản TLC, thời gian từng bậc.

## 3C. Apalache: kiểm tra bước refinement theo quy nạp (tối đa 1 ngày; làm sau 3B nếu đạt L2 trở lên, hoặc nếu bạn muốn thử sớm)

Thầy gợi ý hướng này đáng hơn đẩy TLC. Thay vì khám phá các hành vi tới được, nó hỏi về **một bước từ bất kỳ trạng thái nào thoả bất biến quy nạp `Inv`**: (B1) trạng thái đầu thoả `Inv`; (B2) trạng thái đầu ánh xạ thành trạng thái đầu của đối tượng trừu tượng; (S1) mọi bước cụ thể giữ `Inv`; (S2) mọi bước cụ thể từ trạng thái `Inv` là một bước trừu tượng hoặc làm mọi biến trừu tượng đứng yên. Khi cả bốn đều `NoError` với cùng một `Inv`, đó là một kiểm tra quy nạp ở các hằng số nhỏ cố định của `ApalacheCInit` (N=4, f=1, MAX_ROUND=2, ...), **không phải** chứng minh cho mọi N và không phải bằng chứng cho C1 đầy đủ.

Các file (đã có trong repo; chưa từng được `typecheck` hay chạy):
- `core/MC_ServerRefinementStep_Apalache.tla` (viết tay): ứng viên `Inv`, `IndInit`, `MC_Init`, `MC_Next`, `BadStep`, `StepOK`, `AbsInitOK`.
- `core/EngramConsensus_Apalache.tla`, `core/EngramServerRefinement_Apalache.tla` (sinh bằng `tools/make_apalache_refinement.py`): bản không đệ quy của đối tượng trừu tượng, vì Apalache không hỗ trợ `RECURSIVE` (`SumStakeOp` được viết lại bằng `ApaFoldSet`).

```bash
cd spec
python3 tools/make_apalache_refinement.py && git status --short core        # hai file sinh ra phải không khác bản đã commit
A=~/tools/apalache/apalache/bin/apalache-mc
export JVM_ARGS="-Xmx11g"
```
**Cổng A0 (5–10 phút): kiểu và cú pháp.** Quyết định đi tiếp hay dừng.
```bash
$A typecheck core/MC_ServerRefinementStep_Apalache.tla 2>&1 | tee /tmp/apal_a0.txt
```
- `Type checking completed` thì đi tiếp.
- Dừng và gửi nguyên văn lỗi nếu lỗi nói về `INSTANCE ... WITH`, về dấu nháy `'` trên biểu thức không phải biến (`AbstractConsensus!vars'`), `CHOOSE`, hoặc toán tử không hỗ trợ. Ba rủi ro đã biết: Apalache có thể không hỗ trợ thay biến của `INSTANCE` bằng biểu thức; có thể không cho dấu nháy lên biểu thức; `Q_abstract` dùng `CHOOSE` trên `SUBSET`. Nếu lỗi là một trong các điều đó và không sửa được trong vài vòng, kết luận là "Apalache không biểu diễn được ánh xạ refinement; refinement vẫn chỉ kiểm bằng TLC", đúng như ghi chú đã có ở `EngramServerRefinement.tla`, và dừng mục này.

**Các cổng kiểm tra** (mỗi lệnh tối đa 4 giờ; nếu Apalache từ chối `--length=0` thì dùng `--length=1`):
```bash
D=core/MC_ServerRefinementStep_Apalache.tla
$A check --cinit=ApalacheCInit --init=MC_Init  --next=MC_Next --inv=Inv       --length=0 $D 2>&1 | tee /tmp/apal_b1.txt
$A check --cinit=ApalacheCInit --init=MC_Init  --next=MC_Next --inv=AbsInitOK --length=0 $D 2>&1 | tee /tmp/apal_b2.txt
$A check --cinit=ApalacheCInit --init=IndInit  --next=MC_Next --inv=Inv       --length=1 $D 2>&1 | tee /tmp/apal_s1.txt
$A check --cinit=ApalacheCInit --init=IndInit  --next=BadStep --inv=StepOK    --length=1 $D 2>&1 | tee /tmp/apal_s2.txt
```
Dự kiến: B1 và B2 qua; **S1 và S2 sẽ có phản ví dụ ngay lần đầu**, vì `Inv` hiện chỉ là các bất biến an toàn của trạng thái tới được, chưa mạnh đủ để quy nạp (các trạng thái không tới được thoả `Inv` vẫn có thể có bước hỏng). Đó là việc bình thường của phương pháp này, không phải lỗi.

**Vòng lặp tăng cường (tối đa 6 vòng hoặc 1 ngày Air):**
1. Chạy S1 hoặc S2; khi có phản ví dụ, Apalache ghi vào `./_apalache-out/MC_ServerRefinementStep_Apalache.tla/<thời điểm>/` các file `counterexample1.tla`, `counterexample1.json` và `run.txt`.
2. Gửi cho Claude: lệnh đã chạy, `counterexample1.tla`, 30 dòng cuối của log.
3. Claude đọc phản ví dụ, xác định trạng thái nào không tới được hoặc ràng buộc nào thiếu, viết bổ sung vào `Inv`, commit. Bạn `git pull` rồi chạy lại từ B1.
4. Dừng khi cả bốn cổng `NoError`, hoặc sau 6 vòng, hoặc sau 1 ngày; ghi cổng nào đã qua.

**Claim được phép:**
| Kết quả | Claim |
|---|---|
| A0 không qua và không sửa được | "Apalache cannot express the refinement mapping" (kết quả âm có giá trị; refinement vẫn là TLC-only) |
| B1, B2 qua; S1 hoặc S2 chưa qua sau giới hạn | Không claim; ghi số vòng và loại phản ví dụ còn lại |
| Cả bốn `NoError` | "Inductive check of the refinement step at fixed small constants (N=4, f=1, MAX_ROUND=2, MAX_TIMESTAMP=2, MAX_BTC_HEIGHT=1, MAX_ENGRAM_HEIGHT=1), with an inductive invariant of K conjuncts; not a proof of C1 for all configurations" |

## 4. Cập nhật vào paper (sau khi có số)
- Section 5: bảng `tab:ablation` thêm hàng "DA backfill gate / premature exit before data is published" kèm độ sâu, thời gian, tính chất vi phạm; sửa mô tả (năm hàng thay vì bốn); xoá đoạn ablation f+1 chưa hoàn tất hoặc chuyển thành một câu nói rõ không claim. Nếu các số cũ đổi, thay bằng số mới.
- Section 5 (đoạn liveness và giả thiết Partial synchrony), `spec/README.md` (L1) và Verification Scope: kết quả mục 3A.
- Section 5, Verification Scope: kết quả mục 3B (bậc lớn nhất hoàn tất, gọi là bounded refinement sanity check) và mục 3C (cổng đã qua).
- Section 5, Verification Scope và mục II.A của báo cáo: số C1 mới (cấu hình đã dùng, hoàn tất hay dừng).
- `spec/README.md`: các mục ablation (A–E) và 9.2.3, kèm phiên bản công cụ và ngày chạy; xoá câu "No ablation driver removes the gate yet".
- `docs/NEXT_SESSION_PLAN.md` mục 19 và `docs/report_to_advisor.txt` (mục 6 và II.A).
Gửi lại cho Claude: các log (`/tmp/*.txt`, `/tmp/c1_*.log`), file vết, phiên bản công cụ; Claude cập nhật paper và build.

## 5. Gặp sự cố
- `OutOfMemoryError`: giảm `-Xmx` xuống 9g hoặc dùng `-workers 4`.
- Đĩa đầy: xoá `-metadir`; đặt `-metadir` trên SSD và theo dõi `df -h`.
- Máy nóng, giảm xung: chạy từng việc, không chạy song song TLC với Apalache.
