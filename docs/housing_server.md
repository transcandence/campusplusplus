# Cleaning scheduling algorithm

### Problem statement:

Every day, the housing service must clean many spaces spread over several residences. These spaces fall into two categories:
- **Living Units**: rooms and apartments.
- **Common Areas**: corridors, kitchens and laundry rooms.

Each cleaning is a **task** with an estimated duration. Tasks are carried out by **housekeeper operators**, and each operator works one **cleaning shift** between 08:00 and 17:00 (540 minutes).

The algorithm starts at 20:00 on the day before the cleaning. From then on, cleaning requests keep arriving. Each request is either converted into a task for the next day or placed on a **waiting list**. The algorithm stops once every task that has to be collected is scheduled. From this unordered pool of tasks, it produces **one ordered list of tasks per shift**.

![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=iZdLh8dr|100%]]

The algorithm has three goals:
1. **Feasibility (mandatory):** every task must fit into the available shifts.
2. **Proximity:** the tasks of one shift must be close to each other. Tasks can be in different residences or buildings, and walking between them wastes time.
3. **Fairness:** all operators must receive an equal amount of work

#### 1. Tasks and duration

Each task has a code such as `R12A2-kit` or `R9A2-1`. ==🟠Residence 12, Building A, floor 2, then the unit:== a `room number`, `kit`, `corr` or `apartN`. The task type sets its average time (ATT, average task time): room 30 min, apart 60 min, corridor 15 min, kitchen 45 min, laundry 50 min. These are averages and serve only as a **starting estimate**.

#### 2. Feasibility check

Before any assignment, we check that the work fits into the available time. Let $N$ be **the number of available operators** and $D$ the **duration of one shift** ($D = 540$ min). The total available time is:
$$T_{\text{tot}} = N \times D$$
The time needed to complete the $m$ tasks of the day, where $t_i$ is the duration of task $i$, is:
$$T_r = \sum_{i=1}^{m} t_i$$
We define the error as:
$$\text{Error} = T_{\text{tot}} - T_r$$
- If $\text{Error} \ge 0$, all tasks can be done in time.
- If $\text{Error} < 0$, there is not enough time.

![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=gLwLrDGK|1000]]  

---
#### 3. Sizing the team (worked example)

Consider 5 residences with 5 floors each, which gives  $5 \times 5 = 25$ floors. Each floor has 3 kitchens and 45 rooms:
$$N_{\text{kitchens}} = 25 \times 3 = 75 \qquad N_{\text{rooms}} = 25 \times 45 = 1125$$
Kitchens are cleaned 7 times a week, so all 75 are cleaned every day. Rooms are cleaned once a week, so the daily number of rooms is:
$$\frac{1125}{7} \approx 160 \text{ rooms per day}$$
The daily workload, using the average task times of 30 min per room and 45 min per kitchen, is:
$$T_r = 160 \times 30 + 75 \times 45 = 4800 + 3375 = 8175 \text{ min}$$
Dividing by the shift duration $D = 540$ min gives the minimum number of operators:
$$N_{\min} = \left\lceil \frac{T_r}{D} \right\rceil = \left\lceil \frac{8175}{540} \right\rceil = \lceil 15.14 \rceil = 16$$
With 15 operators, $T_{\text{tot}} = 15 \times 540 = 8100$ min, which is 75 min short. This is a lower bound: it ignores corridors, laundry, apartments, check-outs, travel time and breaks.

![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=o6vTdJT0|99%]]

#### 4. Distributing operators across residences

##### Describing load
The day's workload is split between the $K$ residences by a share $\alpha_k$, where each share is between 0 and 1 and the shares add up to 1:

$$\sum_{k=1}^{K} \alpha_k = 1$$

At the start of the algorithm, $\alpha_k^{0}$ is the portion of dorm units that can be cleaned that day in residence $k$. For example, on day one each of the 5 residences gets:  $$\alpha_k^{0} = 20\%$$

Each residence $k$ is served by a whole number of operators $n_k$, and an operator never travels to another residence. Operators are indivisible, so each $n_k$ must be a whole number, and together they must use exactly the $N$ available operators:

$$n_k \in \mathbb{N} \qquad \sum_{k=1}^{K} n_k = N$$

Residence $k$ therefore has $n_k$ shifts of $D$ minutes each, and its final share of operators is 
$$\alpha_k = n_k / N$$
##### Initialization
The ideal number of operators for residence $k$ is $\alpha_k^{0} N$, which is usually not an integer. We first give each residence the integer part of its share:

$$n_k = \lfloor \alpha_k^{0} N \rfloor$$

The operators that are left over are:

$$r = N - \sum_{k=1}^{K} \lfloor \alpha_k^{0} N \rfloor$$

These $r$ operators are given one each to the $r$ residences with the $r$ residences with the largest fractional part $\{\alpha_k^{0} N\} = \alpha_k^{0} N - \lfloor \alpha_k^{0} N \rfloor$. [Open: tie-break rule when several residences have the same fractional part]

##### Receiving requests
Cleaning requests arrive one at a time, in order of arrival. A request for residence $k$ with estimated duration $t$ becomes a **task** if it fits in one of the $n_k$ shifts of that residence, meaning that some shift still has at least $t$ minutes free. Otherwise it is placed on the **waiting list**.

##### Moving an operator
Operators are moved whole, never in parts. If the accepted tasks of residence $k$ fit in one shift less:

$$\sum_{i \in \text{tasks}(k)} t_i \le (n_k - 1) \times D$$

then one operator is released from $k$ and given to the residence $j$ of the oldest request on the waiting list (first come, first served):

$$n_k \leftarrow n_k - 1 \qquad n_j \leftarrow n_j + 1$$

The total $N$ stays the same. This is repeated while there is a residence that can release an operator and a request that is waiting. [Open: when this happens]






![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=dKqUwfIe|1000]]








![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=A8aTtnQn|99%]]




#### 4. Distributing operators across residences

##### Describing load
The day's workload is split between the $K$ residences by a share $\alpha_k$, where each share is between 0 and 1 and the shares add up to 1:

$$\sum_{k=1}^{K} \alpha_k = 1$$

At the start of the algorithm, $\alpha_k^{0}$ is the portion of dorm units that can be cleaned that day in residence $k$. For example, on day one each of the 5 residences gets:  $$\alpha_k^{0} = 20\%$$

Each residence $k$ is served by a whole number of operators $n_k$, and an operator never travels to another residence. Operators are indivisible, so each $n_k$ must be a whole number, and together they must use exactly the $N$ available operators:

$$n_k \in \mathbb{N} \qquad \sum_{k=1}^{K} n_k = N$$

Residence $k$ therefore has $n_k$ shifts of $D$ minutes each, and its final share of operators is 
$$\alpha_k = n_k / N$$

##### Initialization
The ideal number of operators for residence $k$ is $\alpha_k^{0} N$, which is usually not an integer. We first give each residence the integer part of its share:

$$n_k = \lfloor \alpha_k^{0} N \rfloor$$

The operators that are left over are:

$$r = N - \sum_{k=1}^{K} \lfloor \alpha_k^{0} N \rfloor$$

These $r$ operators are given one each to the $r$ residences with the $r$ residences with the largest fractional part $\{\alpha_k^{0} N\} = \alpha_k^{0} N - \lfloor \alpha_k^{0} N \rfloor$. [Open: tie-break rule when several residences have the same fractional part]

##### Receiving requests
Cleaning requests arrive one at a time, in order of arrival. A request for residence $k$ with estimated duration $t$ becomes a **task** if it fits in one of the $n_k$ shifts of that residence, meaning that some shift still has at least $t$ minutes free. Otherwise it is placed on the **waiting list**.

##### Moving an operator
Operators are moved whole, never in parts. If the accepted tasks of residence $k$ fit in one shift less:

$$\sum_{i \in \text{tasks}(k)} t_i \le (n_k - 1) \times D$$

then one operator is released from $k$ and given to the residence $j$ of the oldest request on the waiting list (first come, first served):

$$n_k \leftarrow n_k - 1 \qquad n_j \leftarrow n_j + 1$$

The total $N$ stays the same. This is repeated while there is a residence that can release an operator and a request that is waiting. [Open: when this happens]