# Chạy nhóm P2P và seven-scenario trên cụm Docker thứ hai (Mac Air M1, 16 GB, 8 lõi)

Mục tiêu: chạy song song hai khối thí nghiệm theo hành vi trên máy thứ hai để máy chính xong sớm hơn khoảng 8 giờ.
Quy tắc: mỗi khối chạy **trọn vẹn trên một máy** (không trộn nhánh của một bảng giữa hai máy); dữ liệu từ Air ghi trong paper là "máy thứ hai (Testbed A')".

| Khối | Script | Thời gian | Nội dung |
|---|---|---|---|
| P2P | `scripts/air/run_p2p.sh` | 4–5 giờ | đối chứng A1/A2 không tấn công ×3, P3 (churn) ×3, P4 (độ trễ relay) ×3, B8 ×3, mỗi lần một chuỗi mới |
| Seven-scenario | `scripts/air/run_seven.sh` | khoảng 5 giờ | S1–S7 ×5 chuỗi mới, heuristic bật, chờ 600 giây giữa các kịch bản |

Chạy **tuần tự** trên Air (P2P trước, seven-scenario sau); cụm chỉ chạy một khối một lúc.

## 1. Cài đặt (1–2 giờ)
1. Docker Desktop (arm64): Settings, Resources, CPU 8, Memory 12 GB, Swap 2 GB, Disk image đủ 100 GB. Tắt ứng dụng khác (trình duyệt) trong lúc chạy.
2. Công cụ: `brew install go python@3.12 git`; cắm điện; tắt chế độ ngủ (`caffeinate -dims &`).
3. Mã nguồn, hai thư mục **cạnh nhau** (go.mod có `replace github.com/cometbft/cometbft => ../engram-consensus-core`):
```bash
cd ~ && git clone git@github.com:cuongct220020/engram-sovereign-fsm.git
git clone git@github.com:cuongct924/engram-consensus-core.git
git -C engram-consensus-core checkout 493a20121caaa5392f5608206a61e2ec41efbd3d     # đúng commit đã dùng trên máy chính
cd engram-sovereign-fsm && git checkout main && git pull
python3 -m venv .venv && .venv/bin/pip install -r requirements.txt 2>/dev/null || .venv/bin/pip install matplotlib seaborn numpy pandas requests
```
4. `.env`: sao chép từ máy chính (không nằm trong git, có thông tin RPC của Bitcoin). Đặt `ENGRAM_PARAM_PROOF_ONLY_RECOVERY=true` (mặc định của bài); `run_seven.sh` tự đổi sang false rồi khôi phục. Kiểm tra `ENGRAM_PARAM_MAX_UNPROVEN_TAIL_BLOCKS=128`, `ENGRAM_PARAM_DOWN_HYSTERESIS_THRESHOLD=2`, `ENGRAM_PARAM_MAX_DOWN_HYSTERESIS_THRESHOLD=2`, `ENGRAM_PARAM_STABLE_WINDOW=16` (cùng giá trị máy chính).
5. Dựng thử một lần và kiểm tra: `scripts/testnet_redeploy_clean.sh engram` (khoảng 12–20 phút lần đầu vì build image), sau đó trong venv:
```bash
python3 - <<'PY'
import sys; sys.path.insert(0,"scripts/framework")
from logger import sample_all_nodes
print([(s.fsm_state, s.height) for s in sample_all_nodes()])
PY
```
Kỳ vọng: 7 node cùng độ cao, chuyển ANCHORED sau khi prover nộp proof đầu (có thể 10–20 phút). Nếu quá 30 phút vẫn không ANCHORED, **dừng** và gửi lại `docker ps`, `docker logs engram-node01 --tail 50`, `docker logs reanchoring-prover --tail 50`.

## 2. Chạy
```bash
cd ~/engram-sovereign-fsm
nohup scripts/air/run_p2p.sh   > ~/p2p.out   2>&1 &      # khoảng 4–5 giờ
# xong (dòng "p2p done" trong ~/engram_air_logs/status.txt) rồi mới:
nohup scripts/air/run_seven.sh > ~/seven.out 2>&1 &      # khoảng 5 giờ
```
Theo dõi: `tail -f ~/engram_air_logs/status.txt`. Mỗi lần chạy lưu log ở `~/engram_air_logs/`.

## 3. Dừng giữa chừng nếu
- Máy rất nóng hoặc `docker stats` cho thấy validator bị throttle dài; ghi lại và gửi.
- Một lần chạy lỗi: script vẫn tiếp tục lần sau; ghi dòng lỗi trong `status.txt`.
- Hết RAM (Docker báo OOM): giảm Memory của Docker xuống 11 GB không giúp; dừng và báo.

## 4. Trả kết quả
Dữ liệu nằm trong các thư mục `results_live` đã có (tên có dấu thời gian, không ghi đè): `e4_p2p_eclipse_detection/results_live` (`sybil_control_*`, `churn_attack_a3_*`, `relay_latency_attack_a4_*`), `e8_attack_resilience/results_live` (`collusion_suite_*`), `e2_fault_injection/results_live/rep_l1..5`. Trên Air:
```bash
git checkout -b air-results
git add scripts/e4_p2p_eclipse_detection/results_live scripts/e8_attack_resilience/results_live scripts/e2_fault_injection/results_live
git commit -m "data: P2P and seven-scenario runs on the second machine" && git push origin air-results
```
Gửi thêm: `~/engram_air_logs/status.txt` và phiên bản (`sw_vers`, `docker version`, cấu hình Docker). Claude sẽ lấy các file dữ liệu (tên khác nhau nên không xung đột) và cập nhật paper, ghi rõ là máy thứ hai.

## 5. Điều chỉnh bên máy chính
Nhóm P2P và seven-scenario sẽ bị **bỏ khỏi hàng đợi máy chính** khi bạn xác nhận Air đã dựng được và bắt đầu chạy (cờ `skip_x`, `skip_rl` do Claude đặt). Nếu Air không chạy được, máy chính vẫn chạy chúng ở cuối hàng đợi như cũ.
