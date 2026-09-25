UPDATE instance_settings
SET storage_quota_disk_percent = 80
WHERE id = 1
  AND storage_quota_disk_percent IS NULL
  AND storage_quota_bytes = 107374182400;
