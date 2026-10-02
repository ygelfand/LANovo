#ifdef SPLAT
void main() {
	vec2 d = gl_PointCoord * 2.0 - 1.0;
	float d2 = dot(d, d);
	if (d2 >= 1.0) discard;
	float q = 1.0 - d2;
	float c = q * q * q;
#ifdef SPLAT_A
	gl_FragColor = vec4(c, c * vcol.a, 0.0, 0.0);
#else
	vec2 g = -6.0 * q * q * d / (vsize * 0.5);
	gl_FragColor = vec4(g, g * vcol.a);
#endif
}
#else

void chrome(highp vec2 uv, out vec4 surf, out vec3 light) {
	surf = vec4(0.0);
	light = vec3(0.0);
	highp vec4 a = texture2D(splatA, uv);
	if (a.r < 0.3) return;
	highp vec4 b = texture2D(splatB, uv);
	highp float F = a.r, Rw = a.g;
	highp vec2 gF = b.xy, gR = b.zw;
	highp float gm = length(gF) + 1e-6;
	float al = clamp(float((F - 0.5) / gm + 0.5), 0.0, 1.0);
	if (al <= 0.0) return;
	highp vec2 gh = vec2(0.0);
	if (F > 0.5) {
		highp float s = sqrt(F - 0.5);
		gh = 1.41 * (gF / (2.0 * s) * (Rw / F) + s * (gR * F - Rw * gF) / (F * F));
	}
	highp float il = inversesqrt(dot(gh, gh) + 1.0);
	float nx = float(-gh.x * il), ny = float(-gh.y * il), nz = float(il);
	vec3 tn = vec3(u[2], u[3], u[4]);
	float rx = 2.0 * nz * nx, up0 = -2.0 * nz * ny, rz = 2.0 * nz * nz - 1.0;
	float up = up0 + 0.2 - 0.3 * rx * rx;
	float sm = clamp((up + 0.12) / 0.24, 0.0, 1.0);
	sm = sm * sm * (3.0 - 2.0 * sm);
	float sq = up > 0.0 ? sqrt(up) : 0.0;
	float gd = up < 0.0 ? min(-up / 0.45, 1.0) : 0.0;
	float ge = 1.0 - gd, ge2 = ge * ge * 0.55;
	vec3 c = vec3(8.0 + 38.0 * ge, 9.0 + 40.0 * ge, 12.0 + 46.0 * ge) + tn * ge2;
	c += (vec3(192.0 - 172.0 * sq, 200.0 - 178.0 * sq, 216.0 - 186.0 * sq) + tn * 0.14 * sq - c) * sm;
	float hz = up / 0.05;
	c += 64.0 * exp(-hz * hz);
	float sbx = rx < -0.8 || rx > -0.1 ? 0.0 : min(1.0, min((rx + 0.8) / 0.22, (-0.1 - rx) / 0.22));
	float sby = up0 < 0.2 || up0 > 0.8 ? 0.0 : min(1.0, min((up0 - 0.2) / 0.2, (0.8 - up0) / 0.2));
	c += sbx * sby * 120.0;
	if (up0 >= -0.3 && up0 <= 0.85) {
		float sx2 = (rx - 0.5) / 0.09;
		c += tn * exp(-sx2 * sx2) * 0.7;
	}
	if (rz < -0.15) {
		float bk = min((-0.15 - rz) / 0.6, 1.0);
		c += (vec3(6.0, 7.0, 10.0) + tn * 0.2 - c) * bk;
	}
	float fr = 1.0 - nz, fr3 = fr * fr * fr;
	c += tn * fr3 * u[10];
	float sp = rx * -0.45 - up0 * -0.62 + rz * 0.64;
	float sv = 0.0;
	if (sp > 0.0) {
		float s2 = sp * sp, s4 = s2 * s2, s8 = s4 * s4, s16 = s8 * s8;
		sv = s16 * s16 * s8 * 340.0;
	}
	surf = vec4(min((c + sv) / 255.0, 1.0) * al, al);
	light = min((vec3(sv * 0.9) + tn * fr3 * u[11]) / 255.0, 1.0) * al;
}

void main() {
	vec4 surf;
	vec3 light;
	chrome(vgl, surf, light);
#ifdef LIGHT
	gl_FragColor = vec4(light, 1.0);
#else
	highp vec2 p = pixel();
	vec3 tn = vec3(u[2], u[3], u[4]) / 255.0;
	float lv = u[5];
	vec3 col = texture2D(t0, vtex).rgb;
	float hd = float(length(p - vec2(u[6], u[7])) / (u[8] * 0.6));
	col += tn * (0.1 + 0.22 * lv) * max(1.0 - hd, 0.0);
	highp vec2 pp = vec2(p.x - u[6], (p.y - u[9]) / 0.14);
	float pd = float(length(pp) / (u[8] * 0.42));
	col += tn * (0.14 + 0.3 * lv) * max(1.0 - pd, 0.0);
	col = surf.rgb + min(col, 1.0) * (1.0 - surf.a);
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
#endif
