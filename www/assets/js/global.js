// jconfirm.defaults = {
//     title: 'Hello',
//     type: 'default',
//     content: 'Are you sure to continue?',
//     buttons: {},
//     defaultButtons: {
//         ok: {
//             action: function () {
//             }
//         },
//         close: {
//             action: function () {
//             }
//         },
//     },
//     contentLoaded: function(data, status, xhr){
//     },
//     icon: '',
//     bgOpacity: null,
//     theme: 'white',
    
//     animationSpeed: 400,
//     animationBounce: 1.2,
//     rtl: false,
//     container: 'body',
//     containerFluid: false,
//     backgroundDismiss: true,
//     backgroundDismissAnimation: 'shake',
//     autoClose: false,
//     closeIcon: null,
//     closeIconClass: 'fa fa-close',
//     columnClass: 'col-md-4 col-md-offset-4 col-sm-6 col-sm-offset-3 col-xs-10 col-xs-offset-1',
//     boxWidth: '50%',
//     useBootstrap: true,
//     bootstrapClasses: {
//         container: 'container',
//         containerFluid: 'container-fluid',
//         row: 'row',
//     },
//     onContentReady: function () {},
//     onOpenBefore: function () {},
//     onOpen: function () {},
//     onClose: function () {},
//     onDestroy: function () {},
//     onAction: function () {}
// };

$(document).ready(function() {
  $(window).on("load",function(){
	$("#menu-nav").mCustomScrollbar({
	  axis: "y",
	  scrollbarPosition: "outside",
	  theme: "dark"
	});
  });
});

function isNumeric(n) {
  return !isNaN(parseFloat(n)) && isFinite(n);
}