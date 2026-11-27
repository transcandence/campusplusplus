# Cleaning scheduling algorithm

### Problem statement

Every day, the housing service must clean many spaces spread across several residences. These spaces fall into two categories:

- **Living Units:** rooms and apartments.
- **Common Areas:** corridors, kitchens and laundry rooms.

Each cleaning is a **task** with an estimated duration. Tasks are carried out by
**housekeeper operators**, and each operator works one **cleaning shift**
between 08:00 and 17:00. A shift consists of two working blocks separated by a
fixed break:

- **Morning block:** 08:00–12:00 (240 minutes)
- **Break:** 12:00–13:00 (the same for all operators; not working time)
- **Afternoon block:** 13:00–17:00 (240 minutes)

A shift therefore provides **D = 480 working minutes**, divided into two blocks
of 240 minutes each.

The system produces **one ordered list of tasks per shift** as its final output
(there are $N$ shifts, one per operator). Production happens in two stages:

- **Algorithm 1** runs live from **20:00 the evening before** until **Algorithm 2 starts at 07:00**. Requests for the next day arrive continuously during this window. Each request is either accepted as a task or placed on a **waiting list**, and the plan is updated in real time. Algorithm 1's output is, **for each residence, the final operator count $n_k$ and the unordered list of accepted tasks** Living Units tagged morning or afternoon, plus Common Areas, which carry no window tag. Algorithm 1 does not order tasks or assign start times.
- **Algorithm 2** *(to be written)* takes that output and produces the ordered list
  of tasks per shift, with start times.

The **system** pursues three goals:

1. **Feasibility (mandatory):** every task must fit within the available shifts.
2. **Proximity:** the tasks of one shift should be close to each other. Tasks may lie in different buildings of the same residence, and walking between them consumes time.
3. **Fairness:** all operators should receive an equal amount of work.

---

### Request-Driven Operator Allocation Across Residences: Handling cleaning requests on the basis of feasibility and available capacity

#### 1. Tasks and duration

Each task has a code such as `R12A2-kit` or `R9A2-1`. 
==Residence 12, Building  A, floor 2, then the unit:== a `room number`, `kit`, `corr` or `apartN`. The task type fixes its average time (ATT, average task time): room 30 min, apartment  60 min, corridor 15 min, kitchen 45 min, laundry 50 min. These are averages and serve only as an initial estimate.

Living Units are cleaned **at most once every 7 days**, counted from the day of the last cleaning rather than from a calendar week. A unit cleaned on Monday becomes due again the following Monday. A request for a unit cleaned less than 7 days ago is **refused** and does **not** enter the waiting list; 

Common Areas are cleaned every day and are tasks from the start; they are never  
requested.
#### 2. Feasibility check

$N$ is the number of operators **available on the day being planned** (it varies with absences) and is an input to the algorithm. Let $D$ be the **duration of a shift**
($D = 480$ min). Total available time is:
$$
  T_{\text{tot}} = N \times D
$$
The time required for the day's work is the sum of all Living Unit tasks **plus**
all Common Areas time:
$$
  T_r = \sum_{i=1}^{m} t_i + \sum_{k=1}^{K} C_k
$$
where $t_i$ is the duration of Living Unit task $i$, and $C_k$ is the constant Common Areas time of residence $k$. $C_k$ is fixed per residence and computed from its Common Areas:
$$C_k = \sum_{c \in k} t_c$$over its corridors, kitchens and laundry rooms. Define the error as:
$$
  \text{Error} = T_{\text{tot}} - T_r
$$
- If $\text{Error} \ge 0$, all tasks fit within the available time.
- If $\text{Error} < 0$, there is not enough time.


---
#### 3. Estimating the team size (rough estimate)

This section gives a quick upper-level estimate of how many operators are needed
in total. It treats all residences as a single pool of work and ignores where
tasks are located. The exact calculation, which accounts for residences, is in
section 4. Section 3 uses averages for a quick bound; section 4 uses actual
durations, which are known.

Consider 5 residences of 5 floors each, giving $5 \times 5 = 25$ floors. Each
floor has 3 kitchens and 45 rooms:

$$
  N_{\text{kitchens}} = 25 \times 3 = 75 \qquad N_{\text{rooms}} = 25 \times 45 = 1125
$$

Kitchens are cleaned 7 times a week, so all 75 are cleaned every day. Rooms are
cleaned once a week, so the daily number of rooms is:

$$
  \frac{1125}{7} \approx 160 \text{ rooms per day}
$$

(Because cleaning is driven by requests and the 7-day clock rather than a daily
quota, this rounding does not lose rooms: a unit not cleaned today remains due
and is cleaned later.)

The daily workload, using average times of 30 min per room and 45 min per
kitchen, is:

$$
  T_r = 160 \times 30 + 75 \times 45 = 4800 + 3375 = 8175 \text{ min}
$$

Dividing by the shift duration $D = 480$ min gives the minimum number of
operators:

$$
  N_{\min} = \left\lceil \frac{T_r}{D} \right\rceil = \left\lceil \frac{8175}{480} \right\rceil = \lceil 17.03 \rceil = 18
$$

With 17 operators, $T_{\text{tot}} = 17 \times 480 = 8160$ min — 15 minutes
short. This estimate is a **lower bound**: it ignores corridors, laundry,
check-outs, travel time and the break being non-working time. It also ignores
the rule that an operator never leaves their residence. With that rule, the real
number can be higher (see section 4).
#### 4. Distributing operators across residences

##### Describing load

The day's workload is divided among the $K$ residences by shares $\alpha_k$, with
each share between 0 and 1 and the shares summing to 1:

$$
  \sum_{k=1}^{K} \alpha_k = 1
$$

At the start of the algorithm, $\alpha_k^{0}$ is residence $k$'s share of the day's **total work** its Living Units still due **plus** its Common Areas.

Because Common Areas are cleaned every day regardless of how many rooms are due, a residence with few due rooms still needs operators for its kitchens and corridors. Since the set of due units is known, the share uses their **actual** durations:

$$
  \alpha_k^{0} = \frac{\sum_{u \in \text{due}(k)} t_u + C_k}{\sum_{j=1}^{K}\left[\sum_{u \in \text{due}(j)} t_u + C_j\right]}
$$

where $\text{due}(k)$ is the set of Living Units in residence $k$ whose last cleaning was 7 or more days ago, $t_u$ is the actual duration of unit $u$, and $C_k$ is the constant Common Areas time of residence $k$. The share uses **all due units**, whether or not they are eventually requested. The acceptance test then filters the actual requests; it does not alter $\alpha_k^0$.

**For example**, with 5 residences of 100 rooms each, on day one all 500 rooms are due and every residence receives 20%. On day two the shares differ: if 25 rooms were cleaned in one residence and only 13 in each of the others, the first residence has fewer due rooms and therefore a smaller share of the Living Unit work (its Common Areas share stays the same).

Each residence $k$ is served by a whole number of operators $n_k$; an operator never travels to another residence. Operators are indivisible, so each $n_k$ must be a whole number and, together, the $n_k$ must use exactly the $N$ available operators:

$$
  n_k \in \mathbb{N} \qquad \sum_{k=1}^{K} n_k = N
$$

Residence $k$ therefore has $n_k$ shifts of $D$ minutes each, and its final
share of operators is

$$
  \alpha_k = n_k / N
$$

$\alpha_k$ is used only for reporting; the algorithm works with the integer
counts $n_k$.

##### Initialization

The ideal number of operators for residence $k$ is $\alpha_k^{0} N$, which is
usually not an integer. We first give each residence the integer part of its
share:

$$
  n_k = \lfloor \alpha_k^{0} N \rfloor
$$

The leftover operators are:

$$
  r = N - \sum_{k=1}^{K} \lfloor \alpha_k^{0} N \rfloor
$$

These $r$ operators are assigned one each to the $r$ residences with the largest
fractional part 
$\{\alpha_k^{0} N\} = \alpha_k^{0} N - \lfloor \alpha_k^{0} N\rfloor$.

Ties are broken by **larger Common Areas time $C_k$**; any remaining ties by
**smaller residence index** (deterministic).

If $N < K$, the configuration is treated as a **degraded mode**: residences are
ranked by **total work** (Living Units due plus Common Areas) in descending
order. The top $N$ receive one operator each; the rest receive $n_k = 0$. The
Common Areas of a residence with $n_k = 0$ are **deferred to the next day** and
added to that residence's $C_k$ on that day, so they count against tomorrow's
capacity. Operationally, $N < K$ should be treated as a configuration error.

##### Receiving requests
Cleaning requests arrive one at a time, in order of arrival, during the request
collection phase. A request for residence $k$ with estimated duration $t$ placed directly on the **waiting list** until that waiting list becomes approximately a shift.
##### Waiting list
At first the residence $k$ has $C_k/D$ shifts' worth of Common Areas. The integer part 
$n_k = \left\lfloor \frac{C_k}{D} \right\rfloor$ is admitted (see below). The leftover Common Area tasks those that did not fit into $n_k$ full shifts stay on the waiting list, together with upcoming Living Unit requests. The waiting list accumulates until it can fill a whole cleaning shift, using the best combination of tasks *(see Admitted list)*.
##### Admitted list
###### 1. For Common Areas
At first it takes the integer part of $C_k/D$, so that, the $k$ residence will acquire $n_k = \lfloor C_k /D \rfloor$ 
and the rest $r=\{C_k/D\}$, stays on the waiting list and adds up to the upcoming request

The system will take those common areas tasks and fill them into the admitted list based on these rules:
- Bigger tasks acquired first.
- The next selected task's duration <= the remaining of $D$
And stops until there no more tasks to take and the some of those common areas $t_i$ divided by $D$ $\approx$ 1.
###### 2. For Living Units
The system will take those waiting list tasks and fill them into the admitted list based on these rules:
- Bigger tasks acquired first.
- The next selected task's duration <= the remaining of $D$
And stops until there no more tasks to take and the some of those common areas $t_i$ divided by $D$ $\approx$ 1.
##### Moving an operatoreee
Operators are moved whole, never in parts. If the accepted work of residence $k$
fits within $n_k - 1$ shifts, one operator is released from $k$ and given to the
residence $j$ of the oldest request on the waiting list (first come, first
served):

$$
  n_k \leftarrow n_k - 1 \qquad n_j \leftarrow n_j + 1
$$

The total $N$ is unchanged. "Fits within $n_k - 1$ shifts" is defined once, in
section 5, and referenced everywhere else.

---
#### 5. Morning and afternoon bookings

##### The residence's capacity
Residence $k$ has $n_k$ operators. Each operator works one shift of $D = 480$ minutes, split into two blocks:

$$
D_M = 240 \quad \text{(morning)}, \qquad D_A = 240 \quad \text{(afternoon)}.
$$

The residence therefore has three independent capacities:

| Capacity               | Symbol       | Value            |
| ---------------------- | ------------ | ---------------- |
| Total work             | $T^{\max}$   | $n_k \times 480$ |
| Morning Living Units   | $T_M^{\max}$ | $n_k \times 240$ |
| Afternoon Living Units | $T_A^{\max}$ | $n_k \times 240$ |

##### The state the engine keeps

Three running totals track what is already admitted:

$$
T^{\text{used}}, \qquad T_M^{\text{used}}, \qquad T_A^{\text{used}}.
$$

- $T^{\text{used}}$ counts **all** admitted work: Common Areas + Living Units of both windows.
- $T_M^{\text{used}}$ counts **only** admitted morning Living Units.
- $T_A^{\text{used}}$ counts **only** admitted afternoon Living Units.

Common Areas count **only** against $T^{\text{used}}$. They do not count against $T_M^{\text{used}}$ or $T_A^{\text{used}}$, because they carry no window and may be placed in either block.

At initialization, Common Areas are admitted first:

$$
T^{\text{used}} = C_k, \qquad T_M^{\text{used}} = 0, \qquad T_A^{\text{used}} = 0.
$$
##### The batch

A batch $B$ is a set of waiting tasks for residence $k$. It is split into three parts:

- $M(B)$ — morning Living Unit tasks
- $A(B)$ — afternoon Living Unit tasks
- $K(B)$ — Common Area tasks (no window)

Its total duration is

$$
t(B) = \sum_{i \in M(B)} t_i + \sum_{i \in A(B)} t_i + \sum_{i \in K(B)} t_i.
$$
##### The four admissibility conditions

A batch $B$ is **admissible** if and only if **all four** conditions hold.

### (C1) Total capacity

$$
T^{\text{used}} + t(B) \le T^{\max}
$$

### (C2) Morning capacity

$$
T_M^{\text{used}} + \sum_{i \in M(B)} t_i \le T_M^{\max}
$$

### (C3) Afternoon capacity


$$
T_A^{\text{used}} + \sum_{i \in A(B)} t_i \le T_A^{\max}
$$


##### Moving an operator (with time windows)

This replaces the basic rule from section 4. An operator is released from
residence $k$ only if **all three** conditions hold:

$$
  \sum_{i \in M(k)} t_i \le (n_k - 1) \times 240
  \qquad
  \sum_{i \in A(k)} t_i \le (n_k - 1) \times 240
  \qquad
  \sum_{i \in M(k)} t_i + \sum_{i \in A(k)} t_i + C_k \le (n_k - 1) \times 480
$$

The first two conditions are required because a released operator frees one
morning block and one afternoon block; if either window still needs more than
$(n_k - 1)$ blocks, the release would leave that window over capacity. The third
condition is the total check. Together they guarantee that the remaining $n_k - 1$
operators can carry the work.

A residence with any work (Living Units or Common Areas) never drops below
$n_k = 1$. $n_k = 0$ is allowed only if the residence has no work at all, which
does not occur in normal operation because Common Areas are cleaned every day.

When the release test passes, one operator is released from $k$ and given to the
residence $j$ of the oldest request on the waiting list (first come, first
served). The total $N$ is unchanged. This repeats while some residence can
release an operator and some request is waiting.

##### Stopping condition and output

Algorithm 1 is **event-driven and uses no clock inside the engine**. It runs live
from **20:00 the evening before** until **Algorithm 2 starts at 07:00**. During
this window, the plan is updated in real time as requests arrive:

- an accepted request is added to its residence's task list;
- a rejected request is placed on the waiting list of its window;
- a cancellation frees capacity and promotes the oldest eligible waitlisted
  request of the same window.

Capacity exhaustion is a **state**, not a stop: when a residence's capacity is
full, further requests for that residence go to the waiting list, and the engine
keeps running in case a cancellation frees space.

At **07:00**, Algorithm 2 starts. At that moment the request stream for today is
**closed**, the plan is **frozen**, and any request arriving after 07:00 joins
**tomorrow's** plan. The final operator counts $n_k$ satisfy $\sum_k n_k = N$.

Algorithm 1's output is, **for each residence**:

- the final operator count $n_k$, and
- the unordered list of accepted tasks: Living Unit tasks tagged **morning** or
  **afternoon**, plus Common Areas tasks, which carry **no window tag**
  (Algorithm 2 may place them in either block).

Algorithm 1 does **not** order tasks, assign them to specific shifts, or set
start times. That is Algorithm 2's job.

### Space and Timing scheduling algorithm
*(Algorithm 2 — to be written: takes Algorithm 1's output — per residence, the
operator count $n_k$ and the unordered task list — and produces one ordered list
of tasks per shift with start times. It defines "close" and "equal work" and
enforces the block rule: a task must start and finish inside a single block and
may not straddle 12:00 or 17:00. It starts at 07:00, which is the moment the
request stream closes and the plan is frozen.)*

![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=iZdLh8dr|100%]]

![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=gLwLrDGK|1000]]  
![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=o6vTdJT0|99%]]
![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=dKqUwfIe|1000]]
![[2- Source-material/Attachment/Excalidraw/Drawing 2026-11-19 18.02.13.excalidraw.md#^frame=A8aTtnQn|99%]]


### Space and Timing scheduling algorithm
*(Algorithm 2 — to be written: takes Algorithm 1's output — per residence, the
operator count $n_k$ and the unordered task list — and produces one ordered list
of tasks per shift with start times. It defines "close" and "equal work" and
enforces the block rule: a task must start and finish inside a single block and
may not straddle 12:00 or 17:00. It starts at 07:00, which is the moment the
request stream closes and the plan is frozen.)*