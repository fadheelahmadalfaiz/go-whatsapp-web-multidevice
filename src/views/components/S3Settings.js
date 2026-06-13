export default {
    name: 'S3Settings',
    data() {
        return {
            enabled: false,
            bucket: '',
            region: 'auto',
            accessKey: '',
            secretKey: '',
            endpoint: '',
            prefix: '',
            publicURL: '',
            retryMax: 3,
            retryWaitMs: 250,
            //
            loading: false,
            testing: false,
            testResult: null,
        }
    },
    methods: {
        async openModal() {
            this.handleReset();
            await this.loadConfig();
            $('#modalS3Settings').modal('show');
        },
        async loadConfig() {
            this.loading = true;
            try {
                let response = await window.http.get('/settings/s3');
                const config = response.data.results;
                this.enabled = config.Enabled || false;
                this.bucket = config.Bucket || '';
                this.region = config.Region || 'auto';
                this.accessKey = config.AccessKey || '';
                this.secretKey = config.SecretKey || '';
                this.endpoint = config.Endpoint || '';
                this.prefix = config.Prefix || '';
                this.publicURL = config.PublicURL || '';
                this.retryMax = config.retry_max || 3;
                this.retryWaitMs = config.retry_wait_ms || 250;
            } catch (error) {
                if (error.response) {
                    showErrorInfo(error.response.data.message);
                } else {
                    showErrorInfo(error.message);
                }
            } finally {
                this.loading = false;
            }
        },
        isValidForm() {
            if (!this.enabled) return true;
            if (!this.bucket.trim()) return false;
            if (!this.accessKey.trim() || !this.secretKey.trim()) return false;
            return true;
        },
        async handleSave() {
            if (!this.isValidForm() || this.loading) {
                return;
            }

            this.loading = true;
            try {
                await this.saveApi();
                showSuccessInfo("S3 configuration saved successfully");
                $('#modalS3Settings').modal('hide');
            } catch (err) {
                showErrorInfo(err);
            } finally {
                this.loading = false;
            }
        },
        async saveApi() {
            const payload = {
                Enabled: this.enabled,
                Bucket: this.bucket,
                Region: this.region,
                AccessKey: this.accessKey,
                SecretKey: this.secretKey,
                Endpoint: this.endpoint,
                Prefix: this.prefix,
                PublicURL: this.publicURL,
                retry_max: this.retryMax,
                retry_wait_ms: this.retryWaitMs,
            };

            try {
                await window.http.put('/settings/s3', payload);
            } catch (error) {
                if (error.response) {
                    throw new Error(error.response.data.message);
                }
                throw new Error(error.message);
            }
        },
        async handleTest() {
            if (!this.enabled) {
                showSuccessInfo("S3 is disabled");
                return;
            }
            if (!this.isValidForm() || this.testing) {
                return;
            }

            this.testing = true;
            this.testResult = null;
            try {
                await this.testApi();
                this.testResult = { success: true, message: "Connection test passed" };
                showSuccessInfo("S3 connection test passed");
            } catch (err) {
                this.testResult = { success: false, message: err };
                showErrorInfo(err);
            } finally {
                this.testing = false;
            }
        },
        async testApi() {
            const payload = {
                Enabled: this.enabled,
                Bucket: this.bucket,
                Region: this.region,
                AccessKey: this.accessKey,
                SecretKey: this.secretKey,
                Endpoint: this.endpoint,
                Prefix: this.prefix,
                PublicURL: this.publicURL,
                retry_max: this.retryMax,
                retry_wait_ms: this.retryWaitMs,
            };

            try {
                await window.http.post('/settings/s3/test', payload);
            } catch (error) {
                if (error.response) {
                    throw new Error(error.response.data.message);
                }
                throw new Error(error.message);
            }
        },
        handleReset() {
            this.enabled = false;
            this.bucket = '';
            this.region = 'auto';
            this.accessKey = '';
            this.secretKey = '';
            this.endpoint = '';
            this.prefix = '';
            this.publicURL = '';
            this.retryMax = 3;
            this.retryWaitMs = 250;
            this.testResult = null;
        }
    },
    template: `
    <div class="olive card" @click="openModal" style="cursor: pointer;">
        <div class="content">
            <a class="ui olive right ribbon label">Settings</a>
            <div class="header">S3 Storage Configuration</div>
            <div class="description">
                Configure S3-compatible object storage (AWS S3, Cloudflare R2, MinIO, etc.)
            </div>
        </div>
    </div>

    <div class="ui large modal" id="modalS3Settings">
        <i class="close icon"></i>
        <div class="header">
            S3 Storage Configuration
        </div>
        <div class="content">
            <form class="ui form">
                <div class="field">
                    <div class="ui checkbox">
                        <input type="checkbox" v-model="enabled">
                        <label>Enable S3 Storage</label>
                    </div>
                </div>

                <div v-if="enabled">
                    <div class="field">
                        <label>Bucket Name *</label>
                        <input type="text" v-model="bucket" placeholder="my-bucket">
                    </div>

                    <div class="field">
                        <label>Region</label>
                        <input type="text" v-model="region" placeholder="auto">
                    </div>

                    <div class="field">
                        <label>Access Key *</label>
                        <input type="text" v-model="accessKey" placeholder="AKIAIOSFODNN7EXAMPLE">
                    </div>

                    <div class="field">
                        <label>Secret Key *</label>
                        <input type="password" v-model="secretKey" placeholder="••••••••••••••••">
                    </div>

                    <div class="field">
                        <label>Endpoint (for R2, MinIO, etc.)</label>
                        <input type="text" v-model="endpoint" placeholder="https://s3.region.amazonaws.com">
                    </div>

                    <div class="field">
                        <label>Object Key Prefix</label>
                        <input type="text" v-model="prefix" placeholder="whatsapp-media/">
                    </div>

                    <div class="field">
                        <label>Public URL Base</label>
                        <input type="text" v-model="publicURL" placeholder="https://cdn.example.com">
                    </div>

                    <div class="two fields">
                        <div class="field">
                            <label>Max Retries</label>
                            <input type="number" v-model="retryMax" min="1" max="10">
                        </div>
                        <div class="field">
                            <label>Retry Wait (ms)</label>
                            <input type="number" v-model="retryWaitMs" min="100" max="5000">
                        </div>
                    </div>
                </div>

                <button type="button" class="ui primary button" :class="{'loading': loading, 'disabled': !this.isValidForm() || this.loading}"
                        @click.prevent="handleSave">
                    Save Configuration
                </button>
                <button type="button" class="ui secondary button" :class="{'loading': testing, 'disabled': !this.enabled || !this.isValidForm() || this.testing}"
                        @click.prevent="handleTest">
                    Test Connection
                </button>

                <div v-if="testResult" class="ui segment" style="margin-top: 1em;">
                    <div :class="testResult.success ? 'ui success message' : 'ui error message'">
                        <div class="header">{{ testResult.success ? 'Success' : 'Failed' }}</div>
                        <p>{{ testResult.message }}</p>
                    </div>
                </div>
            </form>
        </div>
    </div>
    `
}
