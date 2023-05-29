<template>
    <Layout v-loading="loading" :element-loading-text="loadinText" fullscreen>
        <template #menu v-if="!globalStore.isFullScreen">
            <Menu></Menu>
        </template>
        <template #footer v-if="!isXPanelFrame && !globalStore.isFullScreen">
            <Footer></Footer>
        </template>
    </Layout>
</template>
<script setup lang="ts">
import Layout from '@/layout/index.vue';
import Footer from './footer/index.vue';
import Menu from './menu/index.vue';
import { onMounted, computed, ref, watch, onBeforeUnmount } from 'vue';
import { useI18n } from 'vue-i18n';
import { GlobalStore } from '@/store';
import { useTheme } from '@/hooks/use-theme';
import { getSettingInfo, getSystemAvailable, updateSetting } from '@/api/modules/setting';

const i18n = useI18n();
const loading = ref(false);
const loadinText = ref();
const globalStore = GlobalStore();
const themeConfig = computed(() => globalStore.themeConfig);
const { switchDark } = useTheme();

let timer: NodeJS.Timer | null = null;

watch(
    () => globalStore.isLoading,
    () => {
        if (globalStore.isLoading) {
            loadStatus();
        } else {
            loading.value = globalStore.isLoading;
        }
    },
);

const loadDataFromDB = async () => {
    await loadDataFromFrame();
    const res = await getSettingInfo();
    document.title = res.data.panelName;
    i18n.locale.value = res.data.language;
    i18n.warnHtmlMessage = false;
    globalStore.updateLanguage(res.data.language);
    globalStore.setThemeConfig({ ...themeConfig.value, theme: res.data.theme });
    globalStore.setThemeConfig({ ...themeConfig.value, panelName: res.data.panelName });
    switchDark();
};

const loadDataFromFrame = async () => {
    if (!window['x-panel-frame']) {
        return;
    }
    const name = globalStore.urlQueryValue('name') || '管理面板';
    const lang = globalStore.urlQueryValue('lang');
    const theme = globalStore.urlQueryValue('theme');
    // 标题
    if (name && name != globalStore.themeConfig.panelName) {
        globalStore.setThemeConfig({ ...globalStore.themeConfig, panelName: name });
        await updateSetting({ key: 'PanelName', value: name });
    }
    // 主题
    if (theme && theme != globalStore.themeConfig.theme) {
        globalStore.setThemeConfig({ ...globalStore.themeConfig, theme: theme });
        await updateSetting({ key: 'Theme', value: theme });
    }
    // 语言
    if (lang && lang != globalStore.language) {
        globalStore.updateLanguage(lang);
        await updateSetting({ key: 'Language', value: lang });
    }
};

const loadStatus = async () => {
    loading.value = globalStore.isLoading;
    loadinText.value = globalStore.loadingText;
    if (loading.value) {
        timer = setInterval(async () => {
            await getSystemAvailable()
                .then((res) => {
                    if (res) {
                        location.reload();
                        clearInterval(Number(timer));
                        timer = null;
                    }
                })
                .catch(() => {
                    location.reload();
                    clearInterval(Number(timer));
                    timer = null;
                });
        }, 1000 * 10);
    }
};

onBeforeUnmount(() => {
    clearInterval(Number(timer));
    timer = null;
});
onMounted(() => {
    loadStatus();
    loadDataFromDB();
});

const isXPanelFrame = !!window['x-panel-frame'];
</script>
