# Kế hoạch chạy formal trên máy Mac Air M1 (16 GB, 8 lõi, SSD 256 GB)

Mục tiêu: có số liệu formal cuối cùng để cập nhật Section 5, `spec/README.md` và báo cáo, mà không chạm vào cụm Docker ở máy chính.
Không chạy TLC/Apalache trên máy chính khi cụm đang đo.

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
| Circuit breaker | `MC_Ablation_NoCircuitBreakerSafety_Apalache` | `--cinit=ApalacheCInit --init=MC_ServerInit --next=MC_ServerNext --inv=Sanity_NeverAttemptWithdrawalLeakage --length=12` |
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

## 4. Cập nhật vào paper (sau khi có số)
- Section 5: bảng `tab:ablation` thêm hàng "DA backfill gate / premature exit before data is published" kèm độ sâu, thời gian, tính chất vi phạm; sửa mô tả (năm hàng thay vì bốn); xoá đoạn ablation f+1 chưa hoàn tất hoặc chuyển thành một câu nói rõ không claim. Nếu các số cũ đổi, thay bằng số mới.
- Section 5, Verification Scope và mục II.A của báo cáo: số C1 mới (cấu hình đã dùng, hoàn tất hay dừng).
- `spec/README.md`: các mục ablation (A–E) và 9.2.3, kèm phiên bản công cụ và ngày chạy; xoá câu "No ablation driver removes the gate yet".
- `docs/NEXT_SESSION_PLAN.md` mục 19 và `docs/report_to_advisor.txt` (mục 6 và II.A).
Gửi lại cho Claude: các log (`/tmp/*.txt`, `/tmp/c1_*.log`), file vết, phiên bản công cụ; Claude cập nhật paper và build.

## 5. Gặp sự cố
- `OutOfMemoryError`: giảm `-Xmx` xuống 9g hoặc dùng `-workers 4`.
- Đĩa đầy: xoá `-metadir`; đặt `-metadir` trên SSD và theo dõi `df -h`.
- Máy nóng, giảm xung: chạy từng việc, không chạy song song TLC với Apalache.
