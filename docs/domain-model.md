# User Management:

The Identity service manages users, authentication, roles, and profiles. Every
entity belongs to a specific campus tenancy boundary.

### Campus (Tenancy Boundary):

Every user belongs to exactly one **Campus**. - **Campuses**: `rabat`,
`benguerir`, `laayoune`. - The campus code is immutable for a session and is
carried in the JWT claims (`campus_id`) to scope all downstream data (housing,
transport, facilities).

### User & Account:

A **User** represents a real person authenticated on the platform. **Identifiers
& Credentials**: - `email`: unique, case-insensitive (`CITEXT`). -
`phone_number`: unique international phone number. - `password_hash`: securely
hashed password. **Personal Details**: `first_name`, `last_name`, `birthday`,
`gender` (`male`, `female`, `other`), `nationality`. **Account Status**: -
`pending_verification`: account created, pending email/phone verification.
Cannot log in. - `active`: verified and in good standing. Can authenticate and
access authorized services. - `suspended`: temporarily or permanently blocked by
administration. Active sessions revoked; cannot log in. **Affiliation**: The
university entity or school the user belongs to: - `1337`, `ABS`, `FGSES`,
`SHBM`, `SAS`, `CC`, `AEM`, `um6p`.

### Profile & Settings:

- **Profile**: `nickname` (unique display handle), `image` (avatar URL), `bio`.
- **Settings**: `theme` (`light`, `dark`), `language` (`arabic`, `english`,
  `french`), `notification_email` (boolean), `two_factor_enabled` (boolean).
- **Sessions & Authentication**:
  - Short-lived JWT Access Token (10 minutes) with claims: `sub` (user_id),
    `sid` (session_id), `campus_id`, `role`.
  - Long-lived opaque Refresh Token (30 days) with automatic rotation, family
    tracking, and reuse detection.
  - Explicit single-device (`/auth/logout`) or global session revocation
    (`/auth/logout-all`).

### User Roles & Classifications:

Every user has exactly one primary system role (`student`, `worker`, `staff`,
`admin`, `guest`):

1. **Student (`role: student`)**:
   - Belongs to an academic **affiliation** (`1337`, `ABS`, `FGSES`, etc.).
   - Can hold a **Stay** in a housing unit (room or apart).
   - Can request room cleanings, report maintenance issues for their stay, and
     book campus transport.

2. **Worker (`role: worker`)**: Operational campus staff assigned to functional
   teams:
   - **Manager**: administers campus users, approves guest requests, manages
     housing assignments, and accesses reporting.
   - **Housekeeper**:
     - *supervisor*: monitors real-time shift execution, audits completed
       cleanings, and handles exceptions. Shift planning and task assignments
       are fully automated by the platform's scheduling algorithm.
     - *operator*: executes the cleaning-shifts assigned to them by the
       automated system.
   - **Maintenance**: assigned to inspect, service, and resolve maintenance
     tickets.
   - **Driver**: operates scheduled campus buses or on-demand golfettes.

3. **Staff & Admin (`role: staff`, `role: admin`)**:
   - **Staff**: university academic or administrative staff (faculty, department
     coordinators).
   - **Admin**: technical superusers with platform-wide configuration and system
     governance permissions.

4. **Guest (`role: guest`)**: Temporary user invited to campus with a defined
   **stay period** (`arrival_date` → `departure_date`) and a designated **host**
   (a student or worker).
   - **Guest-student**: temporary academic attendee/intern housed on campus;
     receives an assigned **Stay**.
   - **Visitor**: non-housed guest (day visitor, contractor, family); does not
     receive a Stay.

### User Management Rules:

- A user belongs to exactly one campus and has exactly one primary role.
- Only users with status `active` can authenticate and hold active sessions.
- Suspending an account immediately invalidates all active refresh tokens and
  sessions.
- A user can only act as a **host** for a guest if they are an `active` student,
  worker, or staff member.
- Guests cannot act as hosts for other guests.

# Housing:

A **Residence** has several **Buildings**, A **Building** has several
**Floors**. A **Floor** has several **Dorm-units**. 
## Dorm-unit: Every
dorm-unit has a **code**, a **floor**, and a **type**. There are two families:

##### Housing units (people live there, linked to a **Stay**):

- **Room**: capacity of exactly 1 student.
- **Apart**: shared by several students, with a **capacity** greater than 1.
  ##### Common areas (nobody lives there):
- **Corridor**,
- **Kitchen**,
- **Laundry**.

**CleaningShift** - A scheduled cleaning session on a given day and time slot. -
Attributes: - `date` - `timeSlot` - `status`: planned, in progress, or done -
Contains a list of `CleaningTask`, one per dorm unit to clean. **CleaningTask**
- The cleaning of a single dorm unit within a **claeningshift**. - Attributes: -
`dormUnit`: the unit to clean - `status`: planned, in progress, or done -
`doneAt`: time the task was completed (empty until done) - **Automated Planning
& Assignment**: The system's scheduling algorithm automatically groups pending
cleaning tasks by building/floor proximity and assigns each shift to an
available housekeeper *operator*. - **Supervisor Role**: Supervisors monitor the
live execution dashboard, audit completed tasks, reassign in emergency/absence
cases, and verify cleaning quality. - Cleaning of **corridors, kitchens and
laundries** (common areas) is generated automatically on recurring periodic
schedules (e.g., daily/weekly). Students cannot request it. - Cleaning of a
**room or apart** is requested on-demand by the student who holds an active Stay
there, subject to quota and scheduling rules.

### Maintenance:

A **Maintenance request** is raised for one dorm-unit. - **description**,
**priority**, **status** (open, assigned, in progress, resolved), **created
at**, **resolved at**. - Raised by a user (who?), handled by a **maintenance
worker**.

### Laundry:

*(to define)*

### Management:

*(to define)*

### Rules:

- A room has at most one active Stay. An apart has at most as many active Stays
  as its capacity.
- Only housing units can have a Stay. Common areas cannot.
- A cleaning-shift is executed by exactly one operator.
- A cleaning task belongs to exactly one shift and one dorm-unit.

# Transportation:

The transportation service have two type of vehicles, a **Bus** which have only
fixed *departure* and *destination*, or a **Golfette** that have dynamic paths.
### Bus: Every bus has a *schedule* and a *capacity* from which a User-student
can book a **seat**. ### Golf vehicle: every **Golfette** have a *seat-capacity*
and/or a *trunk-capacity*. and a *current-location* every ==🟠permitted user==
can book a **ride** everywhere inside the campus, anytime between the 7h-22h.

### Ride:

have a **pick-up** and a **drop-off** point, a **vehicle** and **passengers**,
