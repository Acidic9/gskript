$(document).ready(function() {
	$("#script-price").autoNumeric('init', {
		aSep: ',',
		aDec: '.',
		aSign: '$ ',
		vMin: '0.00',
		vMax: '99.99'
	});

	$("#script-discount").autoNumeric('init', {
		aSep: ',',
		aDec: '.',
		aSign: '$ ',
		vMin: '0.00',
		vMax: '99.99'
	});

	// $("#script-price").keyup(function() {
	// 	var priceValue = $("#script-price").autoNumeric('get');
	// 	var paypalIncome = (priceValue * 0.039 + 0.3).toFixed(2);
	// 	var userIncome = (priceValue - paypalIncome).toFixed(2);
	// 	if (userIncome < 0) {
	// 		userIncome = "0.00";
	// 	}
	// 	$("#user-income").text(userIncome);
	// 	$("#paypal-income").text(paypalIncome);
	// });

	$("#script-discount").keyup(function() {
		var discountPrice = $("#script-discount").autoNumeric('get');
		var paypalIncome = (discountPrice * 0.039 + 0.3).toFixed(2);
		var userIncome = (discountPrice - paypalIncome).toFixed(2);
		if (userIncome < 0) {
			userIncome = "0.00";
		}
		$("#user-income").text(userIncome);
		$("#paypal-income").text(paypalIncome);
	});

	$("#script-banner").change(function (){
		var fileName = $(this).val().split('\\');
		if (fileName[fileName.length - 1] === "") {
			$("#banner-name").text("No Image Selected");
			return;
		}
		$("#banner-name").text(fileName[fileName.length - 1]);
	});

	$("#script-zip").change(function (){
		var fileName = $(this).val().split('\\');
		if (fileName[fileName.length - 1] === "") {
			$("#zip-name").text("No File Selected");
			return;
		}
		$("#zip-name").text(fileName[fileName.length - 1]);
	});

	var simplemde = new SimpleMDE({
		element: document.getElementById("script-description"),
		placeholder: "Here is some info about my awesome script...",
		showIcons: ["strikethrough", "heading-1", "heading-2", "heading-3", "code", "unordered-list", "ordered-list", "horizontal-rule", "table"],
		hideIcons: ["side-by-side", "fullscreen", "guide"],
		lineWrapping: true
	});

	$("#upload-form").submit(function(e) {
		e.preventDefault();

		if (!validateScript())
			return;

		$("#script-price").val($("#script-price").autoNumeric('get'));
		$("#script-discount").val($("#script-discount").autoNumeric('get'));

		var postData = new FormData();
		postData.append("script-name", $("#script-name").val());
		postData.append("script-description", $("#script-description").val());
		postData.append("script-price", $("#script-price").val());
		postData.append("script-discount", $("#script-discount").val());
		postData.append("script-banner", $("#script-banner").prop('files')[0]);
		postData.append("script-zip", $("#script-zip").prop('files')[0]);

		$(".loading-overlay").show();

		$.ajax({
			url: '/do/upload-script',
			type: 'POST',
			data: postData,
			async: true,
			cache: false,
			contentType: false,
			processData: false,
			dataType: 'json',

			success: function(resp) {
				if (resp.success) {
					swal({
						title: "Successfully Uploaded Script",
						text: "Your script was successfully uploaded.<br>It should be available on the marketplace.",
						type: "success",
						html: true,
					}, function() {
						window.location = '/';
					});
					return;
				}
				
				swal({
					title: "Failed to Upload Script",
					text: resp.err,
					type: "error",
					html: true,
				});
			},

			error: function() {
				swal({
					title: "Failed to Upload Script",
					text: "Something went wrong and we couldn't upload your script.",
					type: "error",
					html: true,
				});
			},

			complete: function() {
				$(".loading-overlay").hide();
			}
		});
	});

	$("#preview-script").click(function(e) {
		e.preventDefault();
		
		if (!validateScript())
			return;

		previewScript();
	});

	function validateScript() {
		// Setup values to variables
		var scriptName = $("#script-name").val(),
			scriptDescription = simplemde.value(),
			sellingPrice = $("#script-price").autoNumeric('get'),
			discountPrice = $("#script-discount").autoNumeric('get'),
			scriptBanner = $("#script-banner").val(),
			scriptZip = $("#script-zip").val();

		// Validate script name
		if (scriptName.length < 4) {
			$("#script-name").css("border-color", "#d42626");
			$("#script-name").css("border-width", "1px");

			$('body').animate({
				scrollTop: $("#script-name").offset().top - 20
			}, 400, function() {
				$("#script-name").focus();
				
				swal({
					title: "Script Name Too Short",
					text: "The script name you have entered is too short.",
					type: "warning"
				});
			});

			$("#script-name").keyup(function() {
				$("#script-name").css("border-color", "initial");
				$("#script-name").css("border-width", "2px");
			});

			return false;
		}

		// Validate scriptDescription
		if (scriptDescription.length < 80) {
			$(".CodeMirror").css("border-color", "#d42626");
			$(".editor-toolbar").css("border-color", "#d42626");

			$('body').animate({
				scrollTop: $(".editor-toolbar").offset().top - 20
			}, 400, function() {
				$(".CodeMirror textarea").focus();
				swal({
					title: "Script Description Too Short",
					text: "The script description you have entered is too short.",
					type: "warning"
				});
			});

			simplemde.codemirror.on("change", function(){
				$(".CodeMirror").css("border-color", "#ddd");
				$(".editor-toolbar").css("border-color", "#ddd");
			});

			return false;
		}

		// Validate sellingPrice
		if (sellingPrice < 1) {
			$("#script-price").css("border-color", "#d42626");
			$("#script-price").css("border-width", "1px");
			
			$('body').animate({
				scrollTop: $("#script-price").offset().top - 20
			}, 400, function() {
				$("#script-price").focus();
				swal({
					title: "Script Price Too Low",
					text: "Your script must sell for at least $1.",
					type: "warning"
				});
			});

			$("#script-price").keyup(function() {
				$("#script-price").css("border-color", "initial");
				$("#script-price").css("border-width", "2px");
			});

			return false;
		}

		// Validate discountPrice
		if (discountPrice < 1) {
			$("#script-discount").css("border-color", "#d42626");
			$("#script-discount").css("border-width", "1px");

			$('body').animate({
				scrollTop: $("#script-discount").offset().top - 20
			}, 400, function() {
				$("#script-discount").focus();
				swal({
					title: "Discount Price Too Low",
					text: "Your script discount price must be at least $1.",
					type: "warning"
				});
			});

			$("#script-discount").keyup(function() {
				$("#script-discount").css("border-color", "initial");
				$("#script-discount").css("border-width", "2px");
			});

			return false;
		}

		// Validate discountPrice is <= sellprice
		if (discountPrice > sellingPrice) {
			$("#script-discount").css("border-color", "#d42626");
			$("#script-discount").css("border-width", "1px");

			$('body').animate({
				scrollTop: $("#script-discount").offset().top - 20
			}, 400, function() {
				$("#script-discount").focus();
				swal({
					title: "Discount Price Invalid",
					text: "Your script's discount price must be less than or equal to the sell price.",
					type: "warning"
				});
			});

			$("#script-discount").keyup(function() {
				$("#script-discount").css("border-color", "initial");
				$("#script-discount").css("border-width", "2px");
			});

			return false;
		}

		if (scriptBanner === "") {
			$('body').animate({
				scrollTop: $("#script-banner").offset().top - 20
			}, 400, function() {
				$("#script-banner").focus();
				swal({
					title: "No Script Banner Image",
					text: "You must choose an image for your script banner.<br>It will be automatically resized to <b>1024px, 256px</b>.",
					type: "warning",
					html: true
				});
			});

			return false;
		}

		if (scriptZip === "") {
			$('body').animate({
				scrollTop: $("#script-zip").offset().top - 20
			}, 400, function() {
				$("#script-zip").focus();
				swal({
					title: "No Script ZIP Selected",
					text: "You must select a ZIP file containing your script.",
					type: "warning"
				});
			});

			return false;
		}

		return true;
	}

	function previewScript() {
		var steamid = $("#steamid").html(),
			profileImg = $("#profileImg").html(),
			bannerImg = window.URL.createObjectURL(document.getElementById("script-banner").files[0]),
			scriptName = $("#script-name").val(),
			scriptPrice = $("#script-price").autoNumeric('get'),
			scriptDiscount = $("#script-discount").autoNumeric('get');

		var previewDiv = `<div style="width:calc(100% - 12px);padding:4px 6px;-webkit-box-shadow:0px 0px 2px 0px rgba(150,150,150,1);-moz-box-shadow:0px 0px 2px 0px rgba(150,150,150,1);box-shadow:0px 0px 2px 0px rgba(150,150,150,1);color: rgb(51, 51, 51);text-align: left;font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif, 'Apple Color Emoji', 'Segoe UI Emoji', 'Segoe UI Symbol';margin-top: -25px;margin-bottom: -19px;">
	<img src="` + bannerImg + `" style="width:100%;height:99px;">
	<a href="/profile/` + steamid + `" style="color:initial;text-decoration:initial;cursor:initial;">
		<img src="` + profileImg + `" title="Script Preview" style="float:left;border-radius:10%;height:44px;width:44px;margin-right:8px;">
	</a>
	<span style="white-space:nowrap;overflow:hidden;text-overflow:ellipsis;display:block;font-weight:bold;text-shadow:#e0e0e0 1px 1px 0;">` + scriptName + `</span>
	<span style="background-color:#57945d;color:#fff;padding:0 4px;font-size: 0.9em;">$` + parseFloat(scriptDiscount).toFixed(2) + `</span>
	<span style="text-decoration:line-through;color: #505050;font-size: 0.88em;">$` + parseFloat(scriptPrice).toFixed(2) + `</span>	
	<span style="float:right;">
		<span>(0)</span>
		<i class="zmdi zmdi-star-outline"></i>
		<i class="zmdi zmdi-star-outline"></i>
		<i class="zmdi zmdi-star-outline"></i>
		<i class="zmdi zmdi-star-outline"></i>
		<i class="zmdi zmdi-star-outline"></i>
	</span>
	<div class="clear"></div>
</div>`;
		
		swal({
			title: null,
			text: previewDiv,
			html: true,
			customClass: "preview-modal"
		});
	}
});