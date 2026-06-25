# Phase 3b — Chunk 2: Backend

## Files
- `new_files/savings_repository.go` — NEW repository for savings_accounts
- `patches/apply_patches.sh` — patches 4 existing Go files via perl/sed
- `docs/README.md` — this file

## Files Modified by patches
1. `internal/adapters/persistence/models/models.go`
   - Add `MastPrindAmt` + `MemberTypeCode` to Flommast struct
   - Add new `SavingsAccount` struct
   - Add `&SavingsAccount{}` to AutoMigrate

2. `internal/adapters/persistence/repositories/interfaces.go`
   - Append `SavingsRepository` interface

3. `internal/adapters/http/handlers/loan_print_handler.go`
   - Add `savingsRepo` field
   - Update constructor signature + struct init
   - Add `MastPrindAmt`/`MemberTypeCode` to MemberFullResponse + mapping
   - Add `GetCollateral` handler + types + constant `CollateralCapPct = 0.95`

4. `internal/adapters/http/routes/routes.go`
   - Init `savingsRepo` and pass to NewLoanPrintHandler
   - Add route `GET /loan-print/collateral/:memb_no`

## Deploy
```bash
# 1. Upload zip to /var/www/loaneasy/phase3b/chunk2/
# 2. Extract:
cd /var/www/loaneasy/phase3b
unzip -o phase3b_chunk2.zip
# 3. Run:
bash chunk2/patches/apply_patches.sh
```

## New endpoint
```
GET /api/v1/loan-print/collateral/:memb_no
Auth: Officer + Admin

Response:
{
  "success": true,
  "message": "Collateral fetched",
  "data": {
    "memb_no": "02428",
    "shares": {
      "value": 6449750.00,
      "max_collateral": 6127262.50
    },
    "savings": [
      {"account_no": "16-XXXXX-X", "full_name": "...", "balance": 1234567.00, "max_collateral": 1172838.65}
    ],
    "total_savings": 1234567.00,
    "total_max_collateral": 7361829.15,
    "cap_pct": 0.95
  }
}
```

## Rollback
```bash
TS=YYYYMMDD-HHMMSS  # from backups/phase3b-XXX/
cp backups/phase3b-${TS}/*.go.bak ... # to original locations
docker compose build app
docker compose up -d app
```
