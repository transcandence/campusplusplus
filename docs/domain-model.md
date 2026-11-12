# User Management:

The Identity service manages users, authentication, roles, and profiles. Every
entity belongs to a specific campus tenancy boundary.

## Campus (Tenancy Boundary):

Every user belongs to exactly one **Campus**. 
- **Campuses**: `rabat`, `benguerir`, `laayoune`. 
- The campus code is immutable for a session and is carried in the JWT claims (`campus_id`) to scope all downstream data (housing, transport, facilities).

## User & Account:

A **User** represents a real person authenticated on the platform. **Identifiers & Credentials**:
- `email`: unique, case-insensitive (`CITEXT`). 
- `phone_number`: unique international phone number.
- `password_hash`: securely hashed password.
##### Personal Details: 
- `first_name`, `last_name`, `birthday`, `gender` (`male`, `female`, `other`), `nationality`. 
##### Account Status: 
- `pending_verification`: account created, pending email/phone verification. Cannot log in.
- `active`: verified and in good standing. Can authenticate and access authorized services.
- `suspended`: temporarily or permanently blocked by administration. Active sessions revoked; cannot log in. 
##### Affiliation: The university entity or school the user belongs to: 
- `1337`, `ABS`, `FGSES`, `SHBM`, `SAS`, `CC`, `AEM`, `um6p`.

## Profile & Settings:

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

## User Roles & Classifications:

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

## User Management Rules:

- A user belongs to exactly one campus and has exactly one primary role.
- Only users with status `active` can authenticate and hold active sessions.
- Suspending an account immediately invalidates all active refresh tokens and
  sessions.
- A user can only act as a **host** for a guest if they are an `active` student,
  worker, or staff member.
- Guests cannot act as hosts for other guests.

# Housing:

The Housing service manages spatial infrastructure, student occupancy (stays),
and cleaning operations.

## Spatial Hierarchy & Unit Code Standard:
```
Campus (Tenancy Boundary: rabat, benguerir, laayoune)
  └── Residence (Numeric code: 8, 12, 20...)
        └── Building (Letter code: A, B, C...)
              └── Floor (Number: 0, 1, 2...)
                    └── DormUnit (Code: [Residence][Building][Floor]-[Unit])
```

- **Unit Code Standard**: `[Residence][Building][Floor]-[Unit]`
  - Living units: `12A2-1`, `8B1-14`
  - Common areas: `12A2-CORR` (Corridor), `12A2-KITCH` (Kitchen), `12A0-LAUND` (Laundry)
- **Unit Types**:
  - **Living Units**:
    - `Room`: Capacity of exactly 1 student.
    - `Apart`: Capacity of N students (shared living unit).
  - **Common Areas** (Capacity = 0, no stays allowed):
    - `Corridor`, `Kitchen`, `Laundry`.
- **Gender Policy**:
  - Enforced at the **Residence level** (`male_only` or `female_only`).
  - A student can only be assigned to a Residence matching their gender, guaranteeing all buildings, floors, and apartments in that residence are single-gender.

---

## Occupancy: The `Stay` Aggregate

A **Stay** records a student's legal and physical occupancy of a LivingUnit over time.

- **Attributes**:
  - `StudentID`: Reference to the authenticated student (`UserID`).
  - `DormUnitID`: Reference to a LivingUnit (`Room` or `Apart`).
  - `Period`: Date interval (`StartDate` → `EndDate`).
  - `Status`:
    - `Planned`: Created and assigned to a student by a **Housing Manager**. Not yet physically on campus.
    - `Active`: Student physically checked in by the building's **Maître d'internat**.
    - `Ended`: Student physically checked out by the **Maître d'internat**.
    - `Cancelled`: Assignment revoked by the Manager prior to check-in.
  - `Actors`: `AssignedBy` (Manager), `CheckedInBy` (Maître d'internat), `CheckedOutBy` (Maître d'internat).

- **Invariants & Business Rules**:
  1. **Single Active Stay**: A student can hold at most **one** `Active` Stay across all campuses at any given time.
  2. **Capacity Limit**: At any given time, the number of `Active` Stays in a unit cannot exceed `DormUnit.Capacity`.
  3. **Living Units Only**: Stays can never be linked to common areas.
  4. **Residence Gender Enforcement**: A student's gender must match the target residence's gender policy.
  5. **Strict Physical Check-out**: A Stay does **not** automatically end when `EndDate` is reached. It remains `Active` until the **Maître d'internat** physically executes the check-out action, preventing overstaying students from disappearing from real-time occupancy counts.

---

## Cleaning Operations Aggregate

Cleaning handles both routine common area sanitization and on-demand student room cleaning.

#### 1. Student On-Demand Cleaning (`CleaningRequest`):
- **Eligibility**: Only students with a currently `Active` Stay can request cleaning for their assigned unit.
- **Weekly Quota**: Max **1 cleaning request per unit per calendar week** (Monday–Sunday).
  - For `Room`: The occupant gets 1 cleaning per week.
  - For `Apart`: Any roommate with an active stay can request it, but it counts for the **entire apartment** (the whole unit gets max 1 request per week).
- **Booking Window & Daily Cut-off**:
  - Students can **only book for tomorrow (the next day)** into open operational slots.
  - Daily booking window closes strictly at **00:00 (midnight)** for next-day cleaning.
- **Operating Hours**: All cleanings are executed between **08:00 and 17:00**.

#### 2. Worker Execution (`CleaningShift` & `CleaningTask`):
- **`CleaningShift`**:
  - Scheduled session assigned to a **Housekeeper Operator** between 08:00 and 17:00.
  - Monitored by a **Housekeeper Supervisor**.
  - Status: `Planned` → `InProgress` → `Completed`.
- **`CleaningTask`**:
  - The cleaning of a single unit within a shift.
  - **Common Areas**: Generated automatically on a recurring schedule without student intervention.
  - **Living Units**: Generated from student `CleaningRequest`s accepted before the 00:00 cut-off.
  - Status: `Pending` → `InProgress` → `Done` / `Skipped`.

---

### Role Responsibilities:

| Role | Key Permissions & Responsibilities |
|---|---|
| **Student** | • View assigned Stay and room details.<br>• Book next-day unit cleaning (1x/week, before 00:00 cut-off).<br>• View cleaning request status. |
| **Maître d'internat** | • Assigned to supervise one or more specific `Building(s)`.<br>• Execute Check-in (`Planned` → `Active`).<br>• Execute physical Check-out (`Active` → `Ended`).<br>• View building unit occupancy and resident list. |
| **Housing Manager** | • Plan and assign Stays to students.<br>• Manage Residence, Building, and Unit inventory.<br>• Cancel planned assignments. |
| **Housekeeper Operator** | • View and execute assigned cleaning tasks between 08:00–17:00.<br>• Mark tasks `InProgress` and `Done`. |
| **Housekeeper Supervisor** | • Monitor real-time shifts and floor cleaning execution.<br>• Handle task reassignments or exceptions. |

---

# Transportation:

The transportation service have two type of vehicles, a **Bus** which have only
fixed *departure* and *destination*, or a **Golfette** that have dynamic paths.
### Bus: Every bus has a *schedule* and a *capacity* from which a User-student
can book a **seat**. ### Golf vehicle: every **Golfette** have a *seat-capacity*
and/or a *trunk-capacity*. and a *current-location* every ==🟠permitted user==
can book a **ride** everywhere inside the campus, anytime between the 7h-22h.

### Ride:

have a **pick-up** and a **drop-off** point, a **vehicle** and **passengers**,
