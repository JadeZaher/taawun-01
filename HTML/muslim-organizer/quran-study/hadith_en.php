<?php

header("Cache-Control: no-store, no-cache, must-revalidate, max-age=0");
header("Pragma: no-cache");
header("Expires: 0");

// <!--
// Quran Arabic Words / Glossary Root (Without Repetition in the quran order - Importance Very Low - No Obligation to learn)
// print("<li><a>Quran Arabic Words / Glossary (Without Repetition in the quran order)</a></li>");
// print("<li><a>Quran Arabic - English Words (Without Repetition in the quran order)</a></li>");
// print("<li><a>13226 Koran Topics</a></li>");
// print("<li><a>Learn free Arabic of the Quran (Not Arabic Secular Language)</a></li>");
// Number of the words of Quran is 78272
// Without repetition is 18454 which is 23%, so we save you 77% of your time to learn quran
// -->


if (!isset($_GET['book']) || $_GET['book'] === 'Hadith') {



	if (!isset($_GET['lang']) || !isset($_GET['book']) || !isset($_GET['mqp'])) {

		header("Location: hadith_en.php?lang=en&book=Hadith&mqp=1");

		exit;

	}



	$mqp_original = $_GET['mqp'];



	$mqp_clean = preg_replace('/[^0-9]/', '', $mqp_original);



	if ($mqp_clean === '') {

		$mqp_clean = '1';

	}



	$mqp = (int) $mqp_clean;



	if ($mqp < 1) {

		$mqp = 1;

	} elseif ($mqp > 1586) {

		$mqp = 1586;

	}



	if (!ctype_digit($mqp_original) || (string)$mqp !== $mqp_original) {

		header("Location: hadith_en.php?lang=" . urlencode($_GET['lang']) . "&book=Hadith&mqp=" . $mqp);

		exit;

	}

}



$version = "2014sep15-0.0.1-2";



ini_set('display_errors', 1);

$debug = false;

$enable_nonverbose_errolog = false;

$page_lang = 'en';

$maxfilesperdir = 100;

$ste_toc_file = "ste.txt";



function errorlog($what_to_log)

{

	return;

	global $enable_nonverbose_errolog;

	$file = "error.txt";

	if ($enable_nonverbose_errolog) {

		print("<pre>");

		print("$what_to_log");

		print("</pre>");

	}

	try {

		$message = $what_to_log . "\n\n";

		error_log($message, 3, $file);

	} catch (Exception $e) {

		print($e->getMessage());

	}

}



function show_buttons($lang, $book, $mqp)

{

	?>

	<p style="text-align:left;">

		<a href="?lang=<?php echo $lang; ?>&book=<?php echo $book; ?>&mqp=<?php echo ($mqp == 1586) ? 1586 : $mqp + 1; ?>#hadith-table">|Next|</a>

		<a href="?lang=<?php echo $lang; ?>&book=<?php echo $book; ?>&mqp=<?php echo ($mqp == 1) ? 1 : $mqp - 1; ?>#hadith-table">|Previous|</a>

		<a href="?lang=<?php echo $lang; ?>&book=<?php echo $book; ?>&mqp=1#hadith-table">|Top|</a>

		<a href="?lang=<?php echo $lang; ?>&book=<?php echo $book; ?>&mqp=1586#hadith-table">|Bottom|</a>

	</p>

	<?php

}



function gen_audio_tags($audio_tag_name, $audio_file_path)

{

	$audio_template = "AudioPlayer.embed(\"$audio_tag_name\", {soundFile: \"$audio_file_path\", autostart: \"yes\", remaining: \"yes\", rtl: \"yes\", buffer: \"10\", initialvolume: 100, transparentpagebg: \"yes\", width: 490});";

	print($audio_template);

}



function substr_unicode($str, $s, $l = null)

{

	return join("", array_slice(preg_split("//u", $str, -1, PREG_SPLIT_NO_EMPTY), $s, $l));

}



function read_topic($heading_only, $headingtxt_path, $audio_tag_name, $pathtoaudiofile)

{

	$mqp = $audio_tag_name;

	global $debug;

	$linenr = 1;

	if (file_exists($headingtxt_path)) {

		$handle = fopen($headingtxt_path, "r");

		if ($handle) {

			$line = fgets($handle);

			$line2 = $line;

			if ($linenr++ == 1) {

				if (!$heading_only) {

					print("<div class=\"heading\">");

				}

				$line_heading_cleaned = mb_ereg_replace("[\|\@]", "", $line2);

				if ($heading_only) {

					return ($line_heading_cleaned);

				}

				print($line_heading_cleaned);

				print("</div>");

				print("<p align=\"left\" id=\"$mqp\"><audio src=\"$pathtoaudiofile\" autoplay controls onended=\"goToNextPage()\"></audio></p>");

			}

			while (($line = fgets($handle)) !== false) {

				$line2 = $line;

				$test = mb_ereg_match(".*#", $line2);

				if ($test) {

					print("<div class=\"quran\">");

					$line_cleaned = mb_ereg_replace("[\#]", "", $line2);

					print($line_cleaned);

					print("</div>");

				} else {

					if (substr_unicode($line2, 0, 1) == "$") {

						print("<div class=\"heading2\">");

						print(substr_unicode($line2, 2, mb_strlen($line2) - 2));

						print("</div>");

					} else {

						print("<div class=\"trans2\">");

						print(substr_unicode($line2, 0, mb_strlen($line2) - 1));

						print("</div>");

					}

				}

			}

			fclose($handle);

		}

	} else {

		if (!$heading_only) {

			print("<h2>Subtitle you are listening is $mqp</h2>");

			print("<p id=\"$mqp\">Audio missing</p>");

			print("<br>");

		}

	}

}



function path_computer_toc_based($audiotoc_file_path, $mqp, $incomplete_audiofilepath)

{

	$delimiter = '|';

	$audio_found = false;

	if (file_exists($audiotoc_file_path)) {

		$handle = fopen($audiotoc_file_path, "r");

		if ($handle) {

			while (($line = fgets($handle)) !== false) {

				$line = trim($line);

				$found_offset = mb_strpos($line, $delimiter);

				if ($found_offset === false) {

					continue;

				}

				list($topicname, $chpdir_audiofile_path) = explode($delimiter, $line);

				if ($topicname == "$mqp") {

					$audio_found = true;

					break;

				}

			}

			fclose($handle);

		}

	}

	$audiofilepath = "$incomplete_audiofilepath/$chpdir_audiofile_path";

	if ($audio_found && file_exists($audiofilepath)) {

		$noaudio = false;

	} else {

		$noaudio = true;

	}

	return array($audiofilepath, $noaudio);

}



function path_computer_for_txt($lang, $book, $mqp)

{

	$txt = "audio-txt";

	$incomplete_txtfilepath = "../../$lang/$book/$txt";

	$mqp_number = (int)$mqp;

	$mqp_4digit = sprintf("%04d", $mqp_number);

	$mqp_4str = strval($mqp_4digit);

	$txtfilename = "NOTFOUND";

	if (file_exists($incomplete_txtfilepath)) {

		$txtdir_ls = scandir($incomplete_txtfilepath);

		foreach ($txtdir_ls as $txtfile) {

			$test = mb_ereg_match("\D*[0]*$mqp_number\D+", $txtfile);

			if ($test) {

				$txtfilename = $txtfile;

				break;

			}

		}

	} else {

		errorlog("<br> Text directory $incomplete_txtfilepath doesn't exist </br>");

	}

	$txtfilepath = "$incomplete_txtfilepath/$txtfilename";

	$notxt = !file_exists($txtfilepath);

	return array($txtfilepath, $notxt);

}



function path_computer($lang, $book, $mqp)

{

	$audio = "audio";

	$AUDIO_TOC_FILE = "ste.txt";

	list($txtfilepath, $notxt) = path_computer_for_txt($lang, $book, $mqp);

	$audiotoc_file_path = "../../$lang/$book/$AUDIO_TOC_FILE";

	if (file_exists($audiotoc_file_path)) {

		$incomplete_audiofilepath = "../../$lang/$book/$audio";

		list($audiofilepath, $noaudio) = path_computer_toc_based($audiotoc_file_path, $mqp, $incomplete_audiofilepath);

	} else {

		$MAXFILES_PER_AUDIODIR = 100;

		$chpdir = ceil($mqp / $MAXFILES_PER_AUDIODIR);

		$audiofilepath = "../../$lang/$book/$audio/$chpdir/$mqp.mp3";

		$noaudio = !file_exists($audiofilepath);

	}

	return array($txtfilepath, $audiofilepath, $notxt, $noaudio);

}



function show_left_pane($lang, $book, $mqp, $nr_of_topics = 5)

{

	print("<b>Next Five topics</b><br>");

	for ($i = $mqp + 1; $i <= $mqp + $nr_of_topics; $i++) {

		list($txtfilepath, $notxt) = path_computer_for_txt($lang, $book, $i);

		$text = read_topic(true, $txtfilepath, "dummy", "dummy");

		print("<div class=\"lpane_topics\"><a href=\"?lang=$lang&book=$book&mqp=$i#hadith-tabel\">$text</a></div>");

	}



	print("<br><strong>Table of Contents</strong><br>");

	$headingtxt_path = "../../$lang/$book/toc/toc.txt";



	if (file_exists($headingtxt_path)) {

		$handle = fopen($headingtxt_path, "r");

		if ($handle) {

			while (($line = fgets($handle)) !== false) {

				$tline = trim($line);

				if (empty($tline)) {

					continue;

				}

				$exploded = explode('=', $tline);

				if (count($exploded) == 2) {

					$chapter_text = $exploded[0];

					$chapter_mqp = $exploded[1];

					print("<div class=\"lpane_topics\"><a href=\"?lang=$lang&book=$book&mqp=$chapter_mqp#hadith-tabel\">$chapter_text</a></div>");

				} else {

					print("bad toc line");

				}

			}

			fclose($handle);

		}

	} else {

		print("There is no TOC available for this book.");

	}

}



mb_internal_encoding("UTF-8");

mb_regex_encoding('UTF-8');



if (isset($_REQUEST['lang'])) {

	$lang = $_REQUEST['lang'];

} else {

	$lang = $page_lang;

}

$book = $_REQUEST['book'];

$mqp = $_REQUEST['mqp'];



list($pathtotxtfile, $pathtoaudiofile, $notxt, $noaudio) = path_computer($lang, $book, $mqp);

?>



<html>



<head>

	<meta http-equiv="Content-Type" content="text/html;charset=utf-8" />

	<title>Allah.com NabiMuhammad.com Muhammad.com</title>

	<script src="https://code.jquery.com/jquery-3.6.0.min.js"></script>

	<script>

		$(document).ready(function() {

			$('.lpane_topics a').click(function(e) {

				e.preventDefault();

				var url = $(this).attr('href');

				$('#right-panel').load(url + ' #right-panel > *');

			});

		});

	</script>

	<script>

	    function goToNextPage() {

	        const currentMqp = <?php echo $mqp; ?>;

	        const nextMqp = (currentMqp == 1586) ? 1586 : currentMqp + 1;

	        window.location.href = `?lang=<?php echo $lang; ?>&book=<?php echo $book; ?>&mqp=${nextMqp}#hadith-table`;

	    }

    </script>

	<style>

		body {

			margin-bottom: 0px;

			padding-bottom: 0px;

		}



		.quran {

			font-family: 'Amiri', 'Scheherazade', 'Roboto', serif !important;

			line-height: 2 !important;

		}



		#top-links {

/*            height: 150px;*/

            background-color: #AABBAA;

            text-align: center;

            padding: 10px;

        }



		.suraName {

			text-align: center;

			font-size: 20px;

			padding: 10px 0px;

			border: 1px solid #D4DDCC;

			background-color: #E4EEDC;

			margin-top: 7px;

		}



		.heading {

			text-align: left;

			font-size: 20px;

			padding: 10px 0px;

			border: 1px solid #D4DDCC;

			margin-top: 7px;

		}

		a {
            color: blue;
        }

		.aya {

			background-color: #EEF8E5;

			border: 1px solid #D4DDCC;

			border-top: 0px;

		}



		.quran {

			font-size: 30px;

			line-height: 140%;

			font-weight: normal;

			direction: ltr;

			text-align: justify;

			border: 0px #e4e4e4 dashed;

		}



		.quran28 {

			font-family: Traditional Arabic;

			font-size: 28px;

			direction: rtl;

			text-align: right;

		}



		.trans_english {

			font-family: Times New Roman;

			font-size: 12px;

			direction: ltr;

			background-color: #E7EFDF;

		}



		.trans2 {

			font-family: Times New Roman;

			font-style: normal;

			font-size: 28px;

			direction: ltr;

		}



		.transurdu-disabled1 {

			font-family: Times New Roman;

			font-size: 20px;

			direction: ltr;

			background-color: #B8B8B8;

		}



		.transurdu-nice {

			font-family: Tahoma, Geneva, sans-serif, Times New Roman;

			font-size: 20px;

			direction: ltr;

			background-color: 669966;

		}



		.transurdu {

			font-family: Verdana, Geneva, sans-serif, Tahoma, Geneva, sans-serif, Times New Roman;

			font-size: 20px;

			direction: ltr;

			background-color: 669966;

		}



		.start {

			font-family: Times New Roman;

			font-size: 20px;

			direction: ltr;

			background-color: 000000;

			color: white;

			text-align: center;

		}



		.heading2 {

			font-family: Times New Roman;

			text-transform: uppercase;

			font-style: normal;

			color: green;

			font-weight: bold;

			font-size: 250%;

			direction: ltr;

		}



		.quran,

		.trans_english,

		.transurdu,

		.start {

			padding: 10px;

		}



		.ayaNum {

			color: green;

			font-size: smaller;

		}



		.sign {

			font-family: times new roman;

			font-size: 0.9em;

			color: #FB7600;

		}



		.footer {

			text-align: center;

			margin: 20px 0px;

			color: #222;

			font-family: Arial;

			background-color: #f4f4ff;

			border: 1px solid #ccd;

			padding: 3px;

			font: 12px Verdana;

		}



		.lpane_topics {
			width: 100%;

			margin-bottom: 10px;
		}



		.hadees {

			border-style: dashed;

			border-width: 5px;

		}



		.hadees-solid {

			border-style: solid;

			border-width: 5px;

		}

	</style>

	<style type="text/css">

		@font-face {

			font-family: 'KFGQPC_Naskh';

			src: url('http://tanzil.net/res/font/eot/KFC_naskh.eot');

			src: local('KFGQPC Uthman Taha Naskh'), url('http://tanzil.net/res/font/org/KFC_naskh.otf') format('opentype');

		}

	</style>

	<style>

		.madnifont-without-quranclass {

			text-align: justify;

			font-family: 'KFGQPC Uthman Taha Naskh', KFGQPC_Naskh;

			font-size: 1.15em;

			font-weight: normal;

		}



		.madnifont {

			text-align: justify;

			font-family: 'KFGQPC Uthman Taha Naskh', KFGQPC_Naskh;

			font-size: 1.15em;

			font-weight: normal;

			font-size: 28px;

			direction: rtl;

			padding: 10px;

			text-align: right;

		}

	</style>

</head>

<body bgcolor="#AABBAA" text="#000000" link="#AA4444" vlink="#DD0000" alink="#FFFF00" topmargin="0" leftmargin="0">
<br><br><br>
	<div id="top-links">
        <a style="text-decoration: none;" href="./assets/The Holy Koran.html" target="_blank">
            <img src="./assets/The Holy Koran.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/Islamic Creed The Articles of Faith.html" target="_blank">
            <img src="./assets/Islamic Creed The Articles of Faith.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/900 years most authorative biography of Prophet Muhammad.html" target="_blank">
            <img src="./assets/900 years most authorative biography of Prophet Muhammad.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/Millennium Biography of Prophet Muhammad.html" target="_blank">
            <img src="./assets/Millennium Biography of Prophet Muhammad.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/Prophet Muhammad Speaks on 61 Topics.html" target="_blank">
            <img src="./assets/Prophet Muhammad Speaks on 61 Topics.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/Baru-Awal-Nur-Cahaya-Muhammad.pdf" target="_blank">
            <img src="./assets/awalnur.jpg" height="265.58">
        </a>

        <br>
        <a style="text-decoration: none;" href="./assets/Kitab Suci Al-Quran.html" target="_blank">
            <img src="./assets/Kitab Suci Al-Quran.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/61 Topik Hadist Nabi Muhammad.html" target="_blank">
            <img src="./assets/61 Topik Hadist Nabi Muhammad.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/Nabi Muhammad Daily Supplications.pdf" target="_blank">
            <img src="./assets/Nabi Muhammad Daily Supplications.jpg" width="191.95">
        </a>
        <a style="text-decoration: none;" href="./assets/Biografi Nabi Muhammad Paling Otentik Sejak 900 Tahun Lalu.html" target="_blank">
            <img src="./assets/Biografi Nabi Muhammad Paling Otentik Sejak 900 Tahun Lalu.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/Milenium Biografi Nabi Muhammad SAW.html" target="_blank">
            <img src="./assets/Milenium Biografi Nabi Muhammad SAW.jpg" width="190.8">
        </a>
        <a style="text-decoration: none;" href="./assets/Naskah_Ihyaus_Sunnah_dan_Dzikir_Al-Nabawiyah.pdf" target="_blank">
            <img src="./assets/ihyasunnah.jpg" width="170.95">
        </a>

        <br>

        <a style="text-decoration: none;" href="./index.php#koran-table">
            <img src="./assets/Quran.png" height="100">
        </a>
    </div>

	<table id="hadith-table" border="1" width="100%" bordercolordark="#FFDD00" bordercolorlight="#AA8800" style="background-color: white;">

		<tr>

			<td align="left" valign="top" width="22%">

				<?php show_left_pane($lang, $book, $mqp, 5); ?>

			</td>

			<td bgcolor="#000000" width="1%"></td>

			<td align="left" valign="top" width="77%" id="right-panel">

				<?php show_buttons($lang, $book, $mqp); ?>

				<?php read_topic(false, $pathtotxtfile, $mqp, $pathtoaudiofile); ?>

				<hr>

				<?php show_buttons($lang, $book, $mqp); ?>

			</td>

		</tr>

	</table>

	<div id="clickPrompt" style="position: fixed; top: 0; left: 0; width: 100%; height: 100%; z-index: 9999; background-color: transparent; pointer-events: none; cursor: default;"></div>

	<div class="gtranslate_wrapper"></div>

	<script>window.gtranslateSettings = {"default_language":"en","languages":["ar","en","id","ru","ur","zh-CN","zh-TW","af","sq","am","hy","as","ay","az","bm","eu","be","bn","bh","bs","bg","ca","ceb","ny","co","hr","cs","da","dv","doi","nl","eo","et","ee","tl","fi","fr","fy","gl","ka","de","el","gn","gu","ht","ha","haw","he","hi","hmn","hu","is","ig","ilo","ga","it","ja","jv","kn","kk","km","rw","kok","ko","kri","ku","ckb","ky","lo","la","lv","ln","lt","lg","lb","mk","mai","mg","ms","ml","mt","mi","mr","mni","lus","mn","my","ne","no","or","om","ps","fa","pl","pt","pa","qu","ro","sm","sa","gd","ns","sr","st","sn","sd","si","sk","sl","so","es","su","sw","sv","tg","ta","tt","te","th","ti","ts","tr","tk","tw","uk","ug","uz","vi","cy","xh","yi","yo","zu"],"wrapper_selector":".gtranslate_wrapper", switcher_horizontal_position:"left", switcher_vertical_position: "top", alt_flags: { en: "usa" }};</script>

	<script src="/float.js" defer></script>

	<script>

        document.addEventListener('DOMContentLoaded', () => {

            const audio = document.querySelector('#right-panel audio');

            const prompt = document.getElementById('clickPrompt');

            const enableAudio = () => {

                if (audio) {

                    audio.muted = false;

                    audio.play().catch(err => console.warn("Playback blocked:", err));

                }

                if (prompt) {

                    prompt.style.display = 'none';

                }

                document.body.removeEventListener('click', enableAudio);

            };

            document.body.addEventListener('click', enableAudio);

        });

    </script>

</body>

</html>

