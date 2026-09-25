-- Wave D: circle invite policy defaults.

ALTER TABLE circles ADD COLUMN invite_who TEXT NOT NULL DEFAULT 'all'
    CHECK (invite_who IN ('all', 'owner'));
ALTER TABLE circles ADD COLUMN invite_kind_default TEXT NOT NULL DEFAULT 'single'
    CHECK (invite_kind_default IN ('single', 'multi'));
