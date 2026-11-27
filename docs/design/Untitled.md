# Batch admission rule (Model A + Model S)

This document defines one rule only: **when a batch of waiting requests is admitted into a residence's plan.**

It assumes:

- **Model A** — requests accumulate on a waiting list and are admitted as a **batch**, not one by one.
- **Model S** — Common Areas float in a shared pool; they are not pre-committed to specific operators' bins.

---

## 1. The residence's capacity

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
|                        |              |                  |

---

## 2. The state the engine keeps

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

---

## 3. The batch

A batch $B$ is a set of waiting tasks for residence $k$. It is split into three parts:

- $M(B)$ — morning Living Unit tasks
- $A(B)$ — afternoon Living Unit tasks
- $K(B)$ — Common Area tasks (no window)

Its total duration is

$$
t(B) = \sum_{i \in M(B)} t_i + \sum_{i \in A(B)} t_i + \sum_{i \in K(B)} t_i.
$$

---

## 4. The four admissibility conditions

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

### (C4) Physical placement of Common Areas

The Common Areas $K(B)$ must be **packable** into the residual gaps of the blocks, after the Living Units of the batch are placed.

The residual gap of a morning block is $240 - (\text{morning Living Units in that block})$, and similarly for the afternoon. $K(B)$ is packable if a first-fit pass places every task of $K(B)$ into some block with sufficient gap.

(C4) is needed because a task cannot be split across blocks. Sums may fit (C1–C3) while no single block has room for one large task.

---

## 5. Admission

When a batch is admissible, **the whole batch is admitted at once**, and the running totals are updated:

$$
T^{\text{used}} \mathrel{+}= t(B), \qquad
T_M^{\text{used}} \mathrel{+}= \sum_{i \in M(B)} t_i, \qquad
T_A^{\text{used}} \mathrel{+}= \sum_{i \in A(B)} t_i.
$$

When a batch is **not** admissible, it stays on the waiting list. It is not admitted piece by piece.

---

## 6. The batch threshold

A batch is **formed** from the waiting list when its total duration reaches a threshold $\theta$:

$$
t(B) \ge \theta.
$$

$\theta$ is a configurable value near $D = 480$. Its role is to prevent half-empty shifts: a batch is admitted only when it is large enough to justify the operator time.

A larger $\theta$ → fuller shifts, but longer waits on the waiting list. A smaller $\theta$ → faster admission, but more half-empty shifts.

---

## 7. Summary

| Check | What it protects |
|---|---|
| (C1) | the residence does not exceed its total operator time |
| (C2) | morning Living Units do not overflow the morning blocks |
| (C3) | afternoon Living Units do not overflow the afternoon blocks |
| (C4) | Common Areas can be physically placed in the remaining block gaps |

The first three are sum checks; the fourth is a placement check. Together they guarantee that an admitted batch is feasible for Algorithm 2 to schedule.