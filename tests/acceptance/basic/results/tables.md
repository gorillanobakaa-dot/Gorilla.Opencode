| job | chatgpt.gpt-5.6-luna / linux | chatgpt.gpt-5.6-luna [fresh-profile] / windows | chatgpt.gpt-5.6-luna / windows | local.qwen3.6-35b-a3b / windows |
|---|---|---|---|---|
| list_folder | PASS (8 s, 1 calls) | PASS (4 s, 1 calls) | PASS (8 s, 1 calls) | PASS (306 s, 1 calls) |
| read_file | PASS (7 s, 1 calls) | PASS (4 s, 1 calls) | PASS (5 s, 1 calls) | PASS (292 s, 1 calls) |
| hidden_file | PASS (15 s, 3 calls) | PASS (8 s, 3 calls) | PASS (10 s, 3 calls) | FAIL (360 s, 0 calls) |
| file_type | PASS (12 s, 2 calls) | PASS (5 s, 2 calls) | PASS (8 s, 2 calls) | not run |
| file_hash | PASS (8 s, 1 calls) | PASS (4 s, 1 calls) | PASS (5 s, 1 calls) | not run |
| create_file | PASS (15 s, 3 calls) | PASS (9 s, 3 calls) | PASS (9 s, 3 calls) | not run |
| make_folders | PASS (24 s, 5 calls) | PASS (10 s, 2 calls) | PASS (9 s, 2 calls) | not run |
| delete_file | PASS (12 s, 2 calls) | PASS (9 s, 3 calls) | PASS (8 s, 3 calls) | PASS (304 s, 1 calls) |
| delete_to_bin | PASS (12 s, 2 calls) | PASS (9 s, 3 calls) | PASS (15 s, 4 calls) | PASS (355 s, 1 calls) |
| rename_file | PASS (15 s, 3 calls) | PASS (7 s, 2 calls) | PASS (6 s, 2 calls) | not run |
| move_file | PASS (20 s, 4 calls) | PASS (9 s, 5 calls) | PASS (11 s, 5 calls) | not run |
| copy_folder | FAIL (16 s, 3 calls) | PASS (8 s, 3 calls) | PASS (11 s, 4 calls) | not run |
| replace_many | PASS (44 s, 9 calls) | PASS (25 s, 9 calls) | PASS (21 s, 10 calls) | not run |
| sort_uniq | PASS (17 s, 3 calls) | FAIL (8 s, 4 calls) | PASS (17 s, 4 calls) | not run |
| zip_make | PASS (23 s, 4 calls) | PASS (10 s, 4 calls) | PASS (11 s, 3 calls) | not run |
| zip_extract | PASS (29 s, 6 calls) | PASS (17 s, 6 calls) | PASS (20 s, 3 calls) | not run |
| sevenzip_make | FAIL (16 s, 3 calls) | PASS (11 s, 5 calls) | PASS (9 s, 3 calls) | not run |
| sevenzip_extract | PASS (26 s, 5 calls) | PASS (158 s, 15 calls) | PASS (218 s, 10 calls) | not run |
| tail_30 | PASS (12 s, 2 calls) | PASS (5 s, 1 calls) | PASS (10 s, 4 calls) | not run |
| follow_log | PASS (25 s, 4 calls) | PASS (14 s, 3 calls) | PASS (21 s, 3 calls) | not run |
| count_lines | PASS (7 s, 1 calls) | PASS (4 s, 1 calls) | PASS (8 s, 1 calls) | not run |
| find_by_name | PASS (8 s, 1 calls) | PASS (4 s, 1 calls) | PASS (9 s, 1 calls) | not run |
| grep_text | PASS (7 s, 1 calls) | PASS (4 s, 1 calls) | PASS (4 s, 1 calls) | not run |
| json_query | PASS (8 s, 1 calls) | PASS (4 s, 1 calls) | PASS (4 s, 1 calls) | not run |
| diff_files | PASS (12 s, 2 calls) | PASS (5 s, 2 calls) | PASS (6 s, 2 calls) | not run |
| count_by_ext | PASS (8 s, 1 calls) | PASS (3 s, 1 calls) | PASS (4 s, 1 calls) | not run |
| disk_usage | PASS (14 s, 2 calls) | PASS (11 s, 2 calls) | PASS (5 s, 1 calls) | not run |
| download_file | PASS (10 s, 1 calls) | PASS (78 s, 10 calls) | PASS (125 s, 9 calls) | not run |
| port_process | PASS (7 s, 1 calls) | PASS (7 s, 1 calls) | PASS (55 s, 4 calls) | not run |
| png_to_webp | PASS (18 s, 3 calls) | PASS (34 s, 8 calls) | PASS (98 s, 13 calls) | not run |
| image_resize | PASS (13 s, 2 calls) | PASS (27 s, 7 calls) | PASS (30 s, 6 calls) | not run |
| ocr_picture | PASS (9 s, 1 calls) | PASS (5 s, 1 calls) | PASS (4 s, 1 calls) | not run |
| exif_read | PASS (15 s, 3 calls) | PASS (17 s, 2 calls) | PASS (14 s, 3 calls) | not run |
| video_length | PASS (7 s, 1 calls) | PASS (61 s, 6 calls) | PASS (44 s, 7 calls) | not run |
| extract_audio | PASS (11 s, 2 calls) | PASS (43 s, 6 calls) | FAIL (57 s, 10 calls) | not run |
| **passed** | **33 of 35** | **34 of 35** | **34 of 35** | **4 of 35** |

| model / OS | job | cause | why it failed |
|---|---|---|---|
| chatgpt.gpt-5.6-luna / linux | copy_folder | model error | Linux run. The copy was made one folder ABOVE the job folder (acc/photos-backup, not copy_folder/photos-backup); the model's answer said it was done. The receipt clips arguments, so whether the model passed a wrong path or the files tool resolved a relative one against another folder was not determined. Same pattern as the Windows sort_uniq failure. |
| chatgpt.gpt-5.6-luna / linux | sevenzip_make | model error | Linux run. docs.7z is a valid-looking output but was written one folder ABOVE the job folder (acc/docs.7z); the model's answer printed that path. 7z was on PATH (needs_present true), so the automatic guess 'model did not know the tool exists' is wrong. Same write-outside-folder pattern as copy_folder. |
| chatgpt.gpt-5.6-luna [fresh-profile] / windows | sort_uniq | model error | Model wrote unique.txt one folder ABOVE the working folder (write path C:\fk-scratch\p5a-acc\unique.txt). The write tool let it; the same job passed in the other run. |
| chatgpt.gpt-5.6-luna / windows | extract_audio | tool missing | ffmpeg is not on the system PATH and not in the embedded toolbox (it is only in scoop here). The model looked, asked the fieldkit videokit tool, which needs a download that needs approval, and stopped to ask. |
| local.qwen3.6-35b-a3b / windows | hidden_file | model error | Killed at the 6-minute job limit with no tool call made: every Qwen job that finished took 292 to 355 s here, even with one tool call, so the limit ended the run while the model was still working (why a turn is that slow was not measured). Not a tool gap. Not re-run (time budget). |

| cause | failures |
|---|---|
| model error | 4 |
| tool missing | 1 |
