package about

// The promise whoever installs Anahita makes, as its first super
// administrator, before setting their password (roadmap row 102, part 2b).
//
// Not the site's rules, which are the operator's (site.json) and agreed to
// by everybody who joins: this is what Anahita's creator, Rastin Mehr, asks of
// whoever runs it (the user, 2026-10-19: "those are my rules as the
// creator"). So it is written here, in the code, and no configuration can
// change it or turn it off.
//
// A promise, not a licence: it speaks to the person's sense of
// responsibility to the people who will trust them, and binds nobody in law
// (the user: "I don't want it to be a legal binding license"). Anahita's
// licences are unchanged. Each item finishes "I won't use Anahita to:", so
// that a tick is a promise not to. The words are the user's, settled
// 2026-10-19.

// PledgeVersion is raised when the promise's words change.
const PledgeVersion = "1"

// PledgeIntro is what Anahita is for, and who the promise is made to.
var PledgeIntro = Text{
	"en-CA":  "Anahita is built to create safe online spaces. The people who join your online space will trust you with their words, their pictures and their communities. Before you begin, please make this promise.",
	"fr-FR":  "Anahita est conçu pour créer des espaces en ligne sûrs. Les personnes qui rejoindront votre espace en ligne vous confieront leurs mots, leurs images et leurs communautés. Avant de commencer, faites cette promesse.",
	"de-DE":  "Anahita ist dafür gebaut, sichere Räume im Netz zu schaffen. Die Menschen, die deinem Online-Raum beitreten, vertrauen dir ihre Worte, ihre Bilder und ihre Gemeinschaften an. Bevor du beginnst, gib bitte dieses Versprechen.",
	"es-419": "Anahita está hecho para crear espacios en línea seguros. Las personas que se unan a tu espacio en línea te confiarán sus palabras, sus imágenes y sus comunidades. Antes de empezar, por favor haz esta promesa.",
	"pt-BR":  "O Anahita foi feito para criar espaços on-line seguros. As pessoas que entrarem no seu espaço on-line vão confiar a você suas palavras, suas imagens e suas comunidades. Antes de começar, por favor faça esta promessa.",
}

// PledgeLead is the sentence each item finishes.
var PledgeLead = Text{
	"en-CA":  "I won't use Anahita to:",
	"fr-FR":  "Je n'utiliserai pas Anahita pour :",
	"de-DE":  "Ich werde Anahita nicht nutzen, um:",
	"es-419": "No usaré Anahita para:",
	"pt-BR":  "Não vou usar o Anahita para:",
}

// PledgeItems is what Anahita is not for.
var PledgeItems = []Text{
	{
		"en-CA":  "Spread hate or organize hate groups, including against women, LGBTQIA+ people, or marginalized communities.",
		"fr-FR":  "Répandre la haine ou organiser des groupes haineux, y compris contre les femmes, les personnes LGBTQIA+ ou les communautés marginalisées.",
		"de-DE":  "Hass zu verbreiten oder Hassgruppen zu organisieren, auch gegen Frauen, LGBTQIA+-Menschen oder marginalisierte Gemeinschaften.",
		"es-419": "Difundir odio u organizar grupos de odio, también contra las mujeres, las personas LGBTQIA+ o las comunidades marginadas.",
		"pt-BR":  "Espalhar ódio ou organizar grupos de ódio, inclusive contra mulheres, pessoas LGBTQIA+ ou comunidades marginalizadas.",
	},
	{
		"en-CA":  "Harm or exploit children.",
		"fr-FR":  "Faire du mal aux enfants ou les exploiter.",
		"de-DE":  "Kindern zu schaden oder sie auszubeuten.",
		"es-419": "Dañar o explotar a niñas y niños.",
		"pt-BR":  "Prejudicar ou explorar crianças.",
	},
	{
		"en-CA":  "Harass, threaten, or expose the private data (dox) of anyone.",
		"fr-FR":  "Harceler, menacer ou exposer les données privées (doxer) de qui que ce soit.",
		"de-DE":  "Jemanden zu belästigen, zu bedrohen oder seine privaten Daten zu veröffentlichen (Doxing).",
		"es-419": "Acosar, amenazar o exponer los datos privados (doxear) de cualquier persona.",
		"pt-BR":  "Assediar, ameaçar ou expor os dados privados (doxxing) de qualquer pessoa.",
	},
	{
		"en-CA":  "Promote violent extremism or terrorism.",
		"fr-FR":  "Promouvoir l'extrémisme violent ou le terrorisme.",
		"de-DE":  "Gewalttätigen Extremismus oder Terrorismus zu fördern.",
		"es-419": "Promover el extremismo violento o el terrorismo.",
		"pt-BR":  "Promover o extremismo violento ou o terrorismo.",
	},
	{
		"en-CA":  "Spread misinformation about health or science.",
		"fr-FR":  "Répandre de la désinformation sur la santé ou la science.",
		"de-DE":  "Falschinformationen über Gesundheit oder Wissenschaft zu verbreiten.",
		"es-419": "Difundir desinformación sobre salud o ciencia.",
		"pt-BR":  "Espalhar desinformação sobre saúde ou ciência.",
	},
	{
		"en-CA":  "Abuse the system through spam, malware, financial scams, or data harvesting.",
		"fr-FR":  "Abuser du système par le spam, les logiciels malveillants, les escroqueries financières ou la collecte de données.",
		"de-DE":  "Das System durch Spam, Schadsoftware, Finanzbetrug oder das Abgreifen von Daten zu missbrauchen.",
		"es-419": "Abusar del sistema con spam, malware, estafas financieras o recolección de datos.",
		"pt-BR":  "Abusar do sistema com spam, malware, golpes financeiros ou coleta de dados.",
	},
}
