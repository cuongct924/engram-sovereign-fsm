# Kế hoạch đóng các khoảng trống của C1 (refinement lớp server sang đối tượng đồng thuận trừu tượng)

Đối tượng cần chứng minh: mỗi bước của lớp server (Tendermint cộng FSM) được ánh xạ (`EngramServerRefinement.tla`, `AbstractConsensus!Safety`) thành một bước của `EngramConsensus` hoặc một bước dừng. Hiện paper không claim C1 và nói rõ đây là nghĩa vụ còn mở.

## Các khoảng trống hiện tại
| # | Khoảng trống | Chi tiết |
|---|---|---|
| G1 | Phần consensus-core (a) không hoàn tất | Dừng sau 1 giờ 41 phút ở 19,8 triệu trạng thái phân biệt, độ sâu 15, còn 13,5 triệu trong hàng đợi; không vi phạm nhưng không phải kết quả |
| G2 | Các bước xen kẽ giữa môi trường và consensus chưa được phủ | (a) đóng băng môi trường, (b) đóng băng consensus; chưa có lập luận kết hợp. Ví dụ `Pull` đọc `fsm_state` và `h_btc_*` đúng lúc hook quyết định ghi trạng thái |
| G3 | Đối tượng đích đã được sửa khi chạy C1 | Thêm `NilQuorumSkipNext`; `ServerHonestTimeout`/`ServerUponTimeoutCert` chỉ chứng nhận vòng không node trung thực nào còn ở; chưa được biện minh trong paper là "đối tượng kiểu LiDO mở rộng" |
| G4 | Ánh xạ có đơn giản hoá | `SUSPICIOUS` gộp vào `ANCHORED`; chỉ một quorum cố định `Q_abstract` có thời gian cục bộ; quyền biểu quyết đều; dịch `h_btc` bằng `K_DEEP`; chưa ghi thành điều kiện của định lý |
| G5 | Quy mô | Chỉ C1 (N=4, f=1). C2 (vòng sâu hơn) và C3 (N=7, f=2) chưa hội tụ |
| G6 | Chỉ safety | Refinement của liveness (`RefinementLiveness`) không nằm trong phạm vi C1 |

## Hướng xử lý, theo thứ tự đáng làm
### Track 1 (G1): một kết quả hoàn tất ở cấu hình nhỏ hơn, có kiểm tra tồn tại
Đã có công cụ: `MC_ConsensusCoreVacuity.tla`, `spec/tools/make_c1_cfgs.py` và các bước 3.1–3.2 của `docs/FORMAL_PLAN_AIR.md`. Tiêu chí: cấu hình phải đạt đổi vòng, proposer Byzantine đề xuất và block được quyết định (kiểm bằng vi phạm của các bất biến `NotReached_*`), và chạy hoàn tất. Kết quả: phần (a) trở thành kết quả hoàn tất có giới hạn.
Công sức: 4–6 giờ máy. Rủi ro thấp.

### Track 2 (G2): chạy nguyên khối (monolithic) ở cấu hình nhỏ đến mức hoàn tất
Đây là cách trực tiếp nhất để phủ các bước xen kẽ mà không cần lập luận kết hợp: chạy `MC_ServerRefinementSafety` (đủ mọi hành động, cả môi trường và consensus) ở các cận nhỏ hơn C1 cho đến khi hoàn tất.
- "Thang cận" (thử lần lượt, mỗi bậc tối đa 6 giờ): L0 `MAX_ROUND=1, MAX_BTC_HEIGHT=1, MAX_ENGRAM_HEIGHT=1, MAX_TIMESTAMP=3`; L1 `MAX_ENGRAM_HEIGHT=2`; L2 `MAX_TIMESTAMP=4`; L3 `MAX_ROUND=2` (= C1 với timestamp nhỏ hơn). Ghi bậc lớn nhất hoàn tất.
- Mỗi bậc phải qua kiểm tra tồn tại cho nguyên khối, gồm thêm hai hành vi xen kẽ: FSM rời `ANCHORED` và có block được quyết định sau đó; đổi vòng. Cần viết `MC_ServerRefinementVacuity.tla` (Claude làm trước khi chạy), mở rộng `make_c1_cfgs.py` nhận driver và cfg gốc khác.
- Kết quả: khẳng định "C1 giữ nguyên khối đến bậc Lk" thay cho phần (a) và (b) rời rạc; phần (a), (b) vẫn giữ làm bằng chứng bổ sung ở cận lớn hơn.
Công sức: 1–2 ngày máy. Rủi ro trung bình: có thể chỉ đạt L0 hoặc L1; khi đó claim rất nhỏ, vẫn phải nói đúng.

### Track 3 (G2, G4): lập luận kết hợp viết tay, chính xác và ngắn
Một phụ lục khoảng một trang, gồm:
1. Tập biến mà ánh xạ đọc: `quorum_certs`, các tập thông điệp nguồn của `MappedTree`, `state`, `round`, `local_rem_time`, `h_btc_current`, `h_btc_anchored`.
2. Với mỗi hành động của `ServerNext`, nó thuộc môi trường hay consensus, ghi biến nào và ánh xạ có đổi không: hành động môi trường (`ServerUpdateEnvironment`, `ServerAdvanceRealTime`, `ServerDABackfill`) hoặc dừng với ánh xạ, hoặc tương ứng một hành động môi trường của đối tượng trừu tượng (đổi `fsm_state` hoặc `h_btc_*`); hành động consensus chỉ ghi `fsm_state` qua hook quyết định `ExecuteFSMTransition`, đúng vị trí mà đối tượng trừu tượng cho phép.
3. Kết luận: một bước xen kẽ bất kỳ là một bước của đúng một loại, nên tính đúng của từng loại (đã kiểm ở (a) và (b)) kéo theo tính đúng của bước đó. Điều kiện duy nhất là hai loại không đọc biến mà loại kia ghi ngoài danh sách ở mục 2; mục 2 liệt kê các ngoại lệ.
Có thể cơ giới hoá từng nghĩa vụ "hành động môi trường dừng với ánh xạ" bằng Apalache một bước từ trạng thái tuỳ ý thoả `TypeOK` (không cần bất biến quy nạp). Claude viết phụ lục sau khi bạn xác nhận danh sách biến; phần cơ giới hoá chỉ làm nếu còn thời gian.
Công sức: 0,5–1 ngày. Rủi ro: lập luận phải đúng; sai sót ở đây nghiêm trọng hơn thiếu kết quả máy.

### Track 4 (G3, G4, G6): nói đúng đối tượng đích và phạm vi
Trong Section 5 viết rõ: đối tượng đích là "LiDO-style object extended with NilQuorumSkipNext", liệt kê từng chỗ lệch so với LiDO (hành động thêm, hạn chế chứng nhận timeout, gộp SUSPICIOUS, một quorum cố định, quyền biểu quyết đều) và nói rằng C1 chỉ là safety. Không cần chạy gì.
Công sức: 1–2 giờ viết.

### Track 5 (G5): không claim; chỉ làm bằng chứng kiểm thử nếu còn thời gian
Chạy TLC `-simulate` (lấy mẫu vết ngẫu nhiên) ở N=7, f=2 (C3) để tìm phản ví dụ, ghi là **kiểm thử, không phải chứng minh**. Quy nạp bằng Apalache (đã nêu "chưa bắt đầu") chỉ làm nếu có ít nhất một ngày: cần bất biến quy nạp và `SumStakeOp` không đệ quy.

## Cái gì sẽ claim được sau các track
- Sau Track 1 và 4: "bounded refinement của lõi đồng thuận ở cấu hình X đã hoàn tất; phần môi trường hoàn tất; C1 đầy đủ còn mở", chặt hơn hiện nay.
- Sau Track 2: nếu bậc L2 trở lên hoàn tất, có thể nói "refinement nguyên khối của lớp server đã hoàn tất ở cận (N=4, f=1, MAX_ROUND=r, …)", và các bước xen kẽ được phủ trong cận đó.
- Sau Track 3: thêm lập luận kết hợp bằng tay cho cận lớn hơn.
- Không claim: C2, C3, liveness refinement, verification đầu-cuối của hệ ghép.

## Thứ tự và phân công
1. Claude: viết `MC_ServerRefinementVacuity.tla` và mở rộng công cụ cấu hình cho nguyên khối (trước khi chạy Track 2). Viết phụ lục Track 3 sau khi có xác nhận.
2. Bạn (máy Air): chạy Track 1 (đã có hướng dẫn), rồi Track 2 theo thang cận, mỗi bậc kèm kiểm tra tồn tại.
3. Claude: cập nhật Section 5, README và báo cáo bằng số thật.

## Trạng thái công cụ (7/10)
- Track 1 và Track 2: sẵn sàng chạy trên Air theo `docs/FORMAL_PLAN_AIR.md` mục 3 và 3B. Đã có `MC_ServerRefinementVacuity.tla` và `make_c1_cfgs.py --layer server --ladder`. Chưa chạy lần nào.
- Track 5 (Apalache): đã có `MC_ServerRefinementStep_Apalache.tla` và `make_apalache_refinement.py` (mục 3C của kế hoạch Air). Chưa `typecheck` lần nào; ba rủi ro đã biết nằm ở cổng A0. Phản hồi thầy (item 7): cấu hình nhỏ chỉ được gọi là bounded refinement sanity check, không phải chứng minh C1.
