# User Management:
Every **User** has one **Account** used to log in, and one **Profile** that describes who they are.

- **Account**: **email** and/or **phone** (at least one), **password**, **status** (active, suspended, archived).
- **Profile**: **name**, **gender**, and exactly one **profile type**: **User-student**, **User-worker** or **User-guest**. A user cannot have two profile types.
### User-student:
- Belongs to exactly one **school**.
- Is housed through a **Stay** (see below).
### User-worker:
Has exactly one **worker role**:
- **Manager**: administers users, housing, reports.
- **Housekeeper**: has a **level**:
  - *supervisor*: plans cleaning-shifts and assigns them to operators.
  - *operator*: executes the cleaning-shifts assigned to them.
- **Maintenance**: handles maintenance requests.
### User-guest:
A temporary user with a **stay period** (arrival → departure) and a **host** (the student or worker who invited them). Can be:
- a **guest-student**: housed on campus, so they get a **Stay**.
- a **visitor**: not housed, no Stay.
### Stay:
Links a student or guest-student to a **dorm-unit** (a room or an apart) for a period.
- **start date**, **end date**, **status** (planned, active, ended).
- Keeps the housing history: who lived where, and when.
### Rules:
- A user has exactly one account and one profile type.
- A student or guest-student has at most one **active** Stay at a time.
- A room has at most one active Stay; an apart can have several, up to its capacity.
- A visitor never has a Stay.
- A cleaning-shift is executed by exactly one housekeeper with the *operator* level.

# Housing:
A **Building** has several **Floors**. A **Floor** has several **Dorm-units**.
### Dorm-unit:
Every dorm-unit has a **code**, a **floor**, and a **type**. There are two families:

- **Housing units** (people live there, linked to a **Stay**):
  - **Room**: capacity of exactly 1 student.
  - **Apart**: shared by several students, with a **capacity** greater than 1.
- **Common areas** (nobody lives there):
  - **Corridor**, **Kitchen**, **Laundry**.

### Cleaning-shift:
- Has a **date**, a **time slot** and a **status** (planned, in progress, done).
- Contains a list of **cleaning tasks**, one per dorm-unit to clean. Each task has its own status and a done-at time.
- Is assigned to exactly one **housekeeper** with the *operator* level, and planned by a *supervisor*.
- Cleaning of **corridors, kitchens and laundries** is only scheduled by the supervisor. Students cannot request it.
- Cleaning of a **room or apart** can be requested by the student who has an active Stay there.

### Maintenance:
A **Maintenance request** is raised for one dorm-unit.
- **description**, **priority**, **status** (open, assigned, in progress, resolved), **created at**, **resolved at**.
- Raised by a user (who?), handled by a **maintenance worker**.

### Laundry:
*(to define)*

### Management:
*(to define)*

### Rules:
- A room has at most one active Stay. An apart has at most as many active Stays as its capacity.
- Only housing units can have a Stay. Common areas cannot.
- A cleaning-shift is executed by exactly one operator.
- A cleaning task belongs to exactly one shift and one dorm-unit.


# Transportation:
The transportation service have two type of vehicles, a **Bus** which have only fixed *departure* and *destination*, or a **Golfette** that have dynamic paths. 
### Bus:
Every bus has a *schedule* and a *capacity* from which a User-student can book a **seat**.
### Golf vehicle:
every **Golfette** have a *seat-capacity* and/or a *trunk-capacity*. and a *current-location*
every ==🟠permitted user== can book a **ride** everywhere inside the campus, anytime between the 7h-22h.

### Ride:
have a **pick-up** and a **drop-off** point, a **vehicle** and **passengers**, 
