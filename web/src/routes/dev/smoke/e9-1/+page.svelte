<!-- Smoke-test: AdminWideLayout from docs/screens.html #e9-1 -->
<script lang="ts">
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import StatusIcon from '$ui/admin/StatusIcon.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';

	const cardStyle =
		'border:1px solid var(--line);background:var(--card);border-radius:12px;padding:16px';
	const descStyle = 'font-size:12.5px;color:var(--muted);margin-bottom:12px;line-height:1.5';
	const sideLabel = 'margin:0;font-size:12.5px;white-space:nowrap';
	const sideLabelRight = 'margin:0 0 0 12px;font-size:12.5px;white-space:nowrap';
	const fillInput = 'width:100%;min-width:0;box-sizing:border-box';
</script>

<AdminWideLayout app height="660px" nav={false}>
	<AdminSection title="Сервер поднят">
		<div style="display:flex;flex-direction:column;gap:16px">
			<div style="display:flex;gap:16px;align-items:stretch">
				<div style="flex:1;min-width:0;{cardStyle}">
					<div style="font-weight:600;margin-bottom:6px">1 · Пароль администратора</div>
					<div style={descStyle}>
						Пароль для доступа в панель администратора. Можно сбросить через консоль.
					</div>
					<Input admin type="password" value="••••••••••••" style="width:100%;box-sizing:border-box" />
				</div>
				<div style="flex:1;min-width:0;{cardStyle}">
					<div style="font-weight:600;margin-bottom:12px">2 · Имя и адрес сервера</div>
					<div style="display:grid;grid-template-columns:auto minmax(0,1fr);gap:8px;align-items:center">
						<SectionLabel raw style={sideLabel}>Имя</SectionLabel>
						<Input admin value="Дом Ани" style={fillInput} />
						<SectionLabel raw style={sideLabel}>Адрес</SectionLabel>
						<Input admin mono value="home.example.org" style={fillInput} />
					</div>
				</div>
			</div>
			<div style={cardStyle}>
				<div style="font-weight:600;margin-bottom:6px">3 · Почта</div>
				<div style={descStyle}>Люди входят по коду из письма. Без настройки SMTP письмо с кодом не отправится.</div>
				<div
					style="display:grid;grid-template-columns:auto minmax(0,1fr) auto minmax(0,1fr);gap:8px;align-items:center"
				>
					<SectionLabel raw style={sideLabel}>Хост</SectionLabel>
					<Input admin mono style={fillInput} value="smtp.selfpost.io" />
					<SectionLabel raw style={sideLabelRight}>Порт</SectionLabel>
					<div style="display:flex;align-items:center;gap:8px;min-width:0">
						<Input admin style="width:72px;flex:none;box-sizing:border-box" value={587} />
						<Hint style="margin:0;white-space:nowrap">587 или 465</Hint>
					</div>
					<SectionLabel raw style={sideLabel}>Логин</SectionLabel>
					<Input admin mono style={fillInput} value="wynd@home.example.org" />
					<SectionLabel raw style={sideLabelRight}>Пароль</SectionLabel>
					<Input admin style={fillInput} type="password" value="secret" />
					<SectionLabel raw style={sideLabel}>От кого</SectionLabel>
					<Input admin mono style={fillInput} value="Wynd <wynd@home.example.org>" />
					<div
						style="grid-column:3/-1;justify-self:end;display:flex;align-items:center;gap:8px"
					>
						<StatusIcon status="ok" style="margin-top:0" />
						<TextButton
							variant="adminBox"
							style="font-weight:600;white-space:nowrap"
							onclick={() => {}}
						>
							Проверить подключение
						</TextButton>
					</div>
				</div>
			</div>
		</div>
		<Button style="margin:20px 0 0;width:100%" onclick={() => {}}>Сохранить и открыть панель</Button>
	</AdminSection>
</AdminWideLayout>
