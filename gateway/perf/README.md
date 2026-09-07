# API Platform Gateway Performance Test Results

<!-- PERF_RESULTS_START -->

**4 Cpu Gateway Runtime**

| Scenario | Users | Throughput | Avg Response Time(ms) | Err % | p90 (ms) | p99 (ms) | Samples |
| --- | --- | --- | --- | --- | --- | --- | --- |
| API Gateway JWT GET | 100 | 7205.05 | 13.69 | 0.00 | 20.00 | 27.00 | 5181719 |
| API Gateway JWT GET | 500 | 7629.20 | 65.18 | 0.00 | 85.00 | 110.00 | 5487063 |
| API Gateway JWT GET | 1000 | 7445.64 | 132.50 | 0.00 | 177.00 | 240.00 | 5357118 |
| API Gateway Plain GET | 100 | 9521.54 | 10.33 | 0.00 | 15.00 | 21.00 | 6849061 |
| API Gateway Plain GET | 500 | 9339.11 | 53.15 | 0.00 | 67.00 | 84.00 | 6716685 |
| API Gateway Plain GET | 1000 | 9202.58 | 108.32 | 0.00 | 133.00 | 163.00 | 6621784 |
| API Gateway Header Policy GET | 100 | 8879.21 | 11.09 | 0.00 | 16.00 | 23.00 | 6386256 |
| API Gateway Header Policy GET | 500 | 8706.58 | 57.19 | 0.00 | 74.00 | 93.00 | 6262678 |
| API Gateway Header Policy GET | 1000 | 8667.24 | 114.66 | 0.00 | 143.00 | 177.00 | 6236016 |

**2 Cpu Gateway Runtime**

| Scenario | Users | Throughput | Avg Response Time(ms) | Err % | p90 (ms) | p99 (ms) | Samples |
| --- | --- | --- | --- | --- | --- | --- | --- |
| API Gateway JWT GET | 100 | 5278.32 | 18.74 | 0.00 | 40.00 | 51.00 | 3796087 |
| API Gateway JWT GET | 500 | 4939.69 | 100.40 | 0.00 | 117.00 | 153.00 | 3553591 |
| API Gateway JWT GET | 1000 | 4972.15 | 200.46 | 0.00 | 229.00 | 265.00 | 3577539 |
| API Gateway Plain GET | 100 | 6149.57 | 16.07 | 0.00 | 35.00 | 45.00 | 4423998 |
| API Gateway Plain GET | 500 | 5707.35 | 87.12 | 0.00 | 105.00 | 137.00 | 4105722 |
| API Gateway Plain GET | 1000 | 5598.03 | 178.28 | 0.00 | 203.00 | 245.00 | 4027863 |
| API Gateway Header Policy GET | 100 | 5751.46 | 17.19 | 0.00 | 37.00 | 47.00 | 4136573 |
| API Gateway Header Policy GET | 500 | 5345.05 | 93.27 | 0.00 | 108.00 | 143.00 | 3845566 |
| API Gateway Header Policy GET | 1000 | 5312.74 | 186.91 | 0.00 | 213.00 | 257.00 | 3822285 |

<!-- PERF_RESULTS_END -->
