# [github.com/cosnicolaou/automation/scheduler/dst_sim](https://pkg.go.dev/github.com/cosnicolaou/automation/scheduler/dst_sim?tab=doc)


// time.Date(2024, 3, 10, 2, 59, 0, 0, America/Los_Angeles) -> 2024-03-10
01:59:00 -0800 PST (isdst: false) // time.Date(2024, 3, 10, 3, 0,
0, 0, America/Los_Angeles) -> 2024-03-10 03:00:00 -0700 PDT (isdst:
true) // // time.Date(2024, 3, 31, 0, 59, 0, 0, Europe/London) ->
2024-03-31 00:59:00 +0000 GMT (isdst: false) // time.Date(2024, 3, 31,
1, 0, 0, 0, Europe/London) -> 2024-03-31 02:00:00 +0100 BST (isdst:
true) // // time.Date(2024, 11, 3, 1, 59, 0, 0, America/Los_Angeles) ->
2024-11-03 01:59:00 -0700 PDT (isdst: true) // time.Date(2024, 11, 3, 2, 0,
0, 0, America/Los_Angeles) -> 2024-11-03 02:00:00 -0800 PST (isdst: false)
// // time.Date(2024, 10, 27, 0, 59, 0, 0, Europe/London) -> 2024-10-27
00:59:00 +0100 BST (isdst: true) // time.Date(2024, 10, 27, 1, 0, 0, 0,
Europe/London) -> 2024-10-27 01:00:00 +0000 GMT (isdst: false) repeating :
ca: 3/1: len: 68: 00:13 00:34 00:55 01:16 01:37 01:58 02:19 02:40 03:01
03:22 03:43 04:04 04:25 04:46 05:07 .. 22:58 23:19 23:40 repeating : ca:
3/10: len: 66: 00:13 00:34 00:55 01:16 01:37 01:58 03:19 03:40 04:01 04:22
04:43 05:04 05:25 05:46 06:07 .. 23:16 23:37 23:58 repeating : ca: 11/3:
len: 71: 00:13 00:34 00:55 01:16 01:37 01:58 01:19 01:40 02:01 02:22 02:43
03:04 03:25 03:46 04:07 .. 23:01 23:22 23:43 ill-defined : ca: 3/1: len:
66: 01:13 01:34 01:55 02:16 02:37 02:58 03:19 03:40 04:01 04:22 04:43 05:04
05:25 05:46 06:07 .. 23:16 23:37 23:58 ill-defined : ca: 3/10: len: 63:
01:13 01:34 01:55 03:16 03:37 03:58 04:19 04:40 05:01 05:22 05:43 06:04
06:25 06:46 07:07 .. 23:13 23:34 23:55 ill-defined : ca: 11/3: len: 66:
01:13 01:34 01:55 02:16 02:37 02:58 03:19 03:40 04:01 04:22 04:43 05:04
05:25 05:46 06:07 .. 23:16 23:37 23:58 ill-defined : uk: 3/1: len: 66:
01:13 01:34 01:55 02:16 02:37 02:58 03:19 03:40 04:01 04:22 04:43 05:04
05:25 05:46 06:07 .. 23:16 23:37 23:58 ill-defined : uk: 3/31: len: 63:
02:13 02:34 02:55 03:16 03:37 03:58 04:19 04:40 05:01 05:22 05:43 06:04
06:25 06:46 07:07 .. 23:13 23:34 23:55 ill-defined : uk: 10/27: len: 66:
01:13 01:34 01:55 02:16 02:37 02:58 03:19 03:40 04:01 04:22 04:43 05:04
05:25 05:46 06:07 .. 23:16 23:37 23:58

