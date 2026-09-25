<!-- Smoke-test: admin components from docs/screens.html #e9-4 -->
<script lang="ts">
	import AdminBar from '$ui/chrome/AdminBar.svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import CheckRow from '$ui/admin/CheckRow.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
</script>

<PhoneFrame shell wide height="940px">
	<AdminBar active="Проверка" server="Дом Ани · home.example.org" />
	<AdminSection>
		<div style="display:flex;align-items:flex-start;gap:20px">
			<div style="flex:1">
				<h4 style="margin-bottom:5px">Две проверки не прошли</h4>
				<div style="font-size:12.5px;color:var(--muted)">
					Обе про прокси и обе чинятся одной вставкой в конфиг. Остальное в порядке.
				</div>
			</div>
			<div style="text-align:right">
				<TextButton variant="admin" style="font-weight:600" onclick={() => {}}>
					Проверить снова
				</TextButton>
				<div style="font-size:11.5px;color:var(--faint);margin-top:7px">проверено минуту назад</div>
			</div>
		</div>
		<div class="cols" style="margin-top:20px">
			<div>
				<SectionLabel style="margin:0 0 6px">Снаружи</SectionLabel>
				<CheckRow
					status="ok"
					name="Домен"
					description="home.example.org ведёт на 203.0.113.10 — адрес этого сервера"
				/>
				<CheckRow
					status="ok"
					name="HTTPS снаружи"
					description="200 за 180 мс; ваш браузер вышел из другой сети"
				/>
				<CheckRow status="ok" name="HTTP → HTTPS" description="308, постоянный" />
				<SectionLabel style="margin:16px 0 6px">Сертификат</SectionLabel>
				<CheckRow
					status="ok"
					name="Let’s Encrypt"
					description="до 12 ноября, 84 дня; продлевает Caddy"
				/>
				<CheckRow status="ok" name="Цепочка" description="полная — откроют и старые Android" />
				<SectionLabel style="margin:16px 0 6px">Прокси</SectionLabel>
				<CheckRow
					status="bad"
					bad
					name="Настоящий адрес клиента"
					description="X-Forwarded-For не приходит: «три попытки» на код считаются всем сразу"
				>
					{#snippet actions()}
						<TextButton variant="admin" onclick={() => {}}>Показать конфиг</TextButton>
					{/snippet}
				</CheckRow>
				<CheckRow
					status="bad"
					bad
					name="Потолок тела запроса"
					description="1 МБ: фотография 2048 px вернётся с 413"
				>
					{#snippet actions()}
						<TextButton variant="admin" onclick={() => {}}>Показать конфиг</TextButton>
					{/snippet}
				</CheckRow>
				<CheckRow
					status="ok"
					name="Протокол"
					description="X-Forwarded-Proto: https — ссылки в письмах верные"
				/>
				<CheckRow status="ok" name="Буферизация" description="выключена, SSE доходит сразу" />
				<CheckRow status="ok" name="Таймаут" description="300 с, длинная загрузка не рвётся" />
			</div>
			<div>
				<SectionLabel style="margin:0 0 6px">Почта</SectionLabel>
				<CheckRow status="ok" name="SMTP" description="smtp.selfpost.io, вход принят, 130 мс">
					{#snippet actions()}
						<TextButton variant="admin" onclick={() => {}}>Настроить</TextButton>
					{/snippet}
				</CheckRow>
				<CheckRow status="ok" name="Тестовое письмо" description="дошло 2 минуты назад">
					{#snippet actions()}
						<TextButton variant="admin" onclick={() => {}}>Отправить ещё</TextButton>
					{/snippet}
				</CheckRow>
				<CheckRow
					status="warn"
					name="DKIM"
					description="записи нет: письмо с кодом уедет в спам, а пароля у участников нет"
				>
					{#snippet actions()}
						<TextButton variant="admin" onclick={() => {}}>Как добавить</TextButton>
					{/snippet}
				</CheckRow>
				<SectionLabel style="margin:16px 0 6px">Пуши</SectionLabel>
				<CheckRow status="ok" name="VAPID-ключи" description="созданы при установке">
					{#snippet actions()}
						<TextButton variant="admin" onclick={() => {}}>Скачать копию</TextButton>
					{/snippet}
				</CheckRow>
				<CheckRow status="ok" name="Тестовый пуш" description="дошёл до этого браузера">
					{#snippet actions()}
						<TextButton variant="admin" onclick={() => {}}>Отправить</TextButton>
					{/snippet}
				</CheckRow>
				<SectionLabel style="margin:16px 0 6px">Приложение</SectionLabel>
				<CheckRow
					status="ok"
					name="Манифест и service worker"
					description="PWA ставится, камера открывается"
				/>
				<SectionLabel style="margin:16px 0 6px">Сервер</SectionLabel>
				<CheckRow
					status="ok"
					name="Часы"
					description="расходятся с NTP на 0,4 с; код живёт 15 минут"
				/>
				<CheckRow status="ok" name="Место" description="свободно 21,6 ГБ из 40" />
				<CheckRow status="ok" name="Суточная рутина" description="прошла сегодня в 04:00" />
				<CheckRow
					status="warn"
					name="Бэкап"
					description="каталог ещё ни разу не копировали"
				>
					{#snippet actions()}
						<TextButton variant="admin" onclick={() => {}}>Как настроить</TextButton>
					{/snippet}
				</CheckRow>
			</div>
		</div>
		<div style="font-size:11.5px;color:var(--faint);margin-top:18px;line-height:1.6">
			Снаружи проверяет ваш браузер: панель просит его сходить на публичный адрес и сравнивает с тем, что видит сервер. Wynd никуда не звонит — если браузер окажется в одной сети с сервером, здесь будет сказано, что проверка вышла изнутри.
		</div>
	</AdminSection>
</PhoneFrame>
